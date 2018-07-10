package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strconv"

	"merkle-tree-and-bitcoin/hash"
	"merkle-tree-and-bitcoin/merkle"
)

// AuxMerkleBranch 合并挖矿的 Merkle Branch
type AuxMerkleBranch struct {
	branchs  []hash.Byte32
	sideMask uint32
}

// AuxPowData 辅助工作量数据
type AuxPowData struct {
	coinbaseTxn      []byte
	blockHash        hash.Byte32
	coinbaseBranch   AuxMerkleBranch
	blockchainBranch AuxMerkleBranch
	parentBlock      []byte
}

// ParseAuxPowData 解析辅助工作量数据
/*
<https://en.bitcoin.it/wiki/Merged_mining_specification#Aux_proof-of-work_block>

 ? coinbase_txn         txn             Coinbase transaction that is in the parent block, linking the AuxPOW block to its parent block.
32 block_hash           char[32]        Hash of the parent_block header.
 ? coinbase_branch      Merkle branch   The merkle branch linking the coinbase_txn to the parent block's merkle_root.
 ? blockchain_branch    Merkle branch   The merkle branch linking this auxiliary blockchain to the others,
                                        when used in a merged mining setup with multiple auxiliary chains.
80 parent_block         Block header    Parent block header.
*/
func ParseAuxPowData(dataHex string) (auxPowData *AuxPowData, err error) {
	auxPowData = new(AuxPowData)

	data, err := hex.DecodeString(dataHex)
	if err != nil {
		return
	}

	if len(data) <= 80 {
		err = errors.New("AuxPowData should be more than 80 bytes")
		return
	}

	// 80 bytes of parent block header
	auxPowData.parentBlock = make([]byte, 80)
	copy(auxPowData.parentBlock, data[len(data)-80:])

	// 因为解析 coinbase_txn 十分困难，且无法简单得到其准确长度，
	// 所以决定先计算出 block_hash，然后从字节流中找到该 hash 以确定 coinbase_txn 的长度。
	auxPowData.blockHash = hash.Hash(auxPowData.parentBlock)
	// BTCPool的默认字节序是 big-endian
	auxPowData.blockHash = auxPowData.blockHash.Reverse()

	// 从字节流中找到 block_hash 以确定 coinbase_txn 的长度
	index := bytes.Index(data, auxPowData.blockHash[:])
	if index == -1 {
		/* 找不到，尝试 little-endian
		* <https://en.bitcoin.it/wiki/Merged_mining_specification#Aux_proof-of-work_block>
		* Note that the block_hash element is not needed as you have the full parent_block header element
		* and can calculate the hash from that. The current Namecoin client doesn't check this field for
		* validity, and as such some AuxPOW blocks have it little-endian, and some have it big-endian.
		 */
		auxPowData.blockHash = auxPowData.blockHash.Reverse()
		index = bytes.Index(data, auxPowData.blockHash[:])
		if index == -1 {
			err = errors.New("cannot found blockHash " + auxPowData.blockHash.Hex() + " from AuxPowData " + dataHex)
			return
		}
	}

	// index 在数值上等于 coinbase_txn 的长度
	auxPowData.coinbaseTxn = make([]byte, index)
	copy(auxPowData.coinbaseTxn, data[0:])

	// 跳过 block_hash
	index += 32

	// coinbaseBranchSize 为变长整数 <https://en.bitcoin.it/wiki/Protocol_documentation#Variable_length_integer> ，
	// 但是不太可能超过 0xFD。所以假设 coinbaseBranchSize 只有一字节。
	coinbaseBranchSize := int(data[index])
	index++

	// 读取 coinbase branch
	auxPowData.coinbaseBranch.branchs = make([]hash.Byte32, coinbaseBranchSize)
	for i := 0; i < coinbaseBranchSize; i++ {
		copy(auxPowData.coinbaseBranch.branchs[i][:], data[index:])
		index += 32
	}

	// 读取 coinbase branch 的 side mask
	sideMask := make([]byte, 4)
	copy(sideMask, data[index:])
	auxPowData.coinbaseBranch.sideMask = binary.LittleEndian.Uint32(sideMask)
	index += 4

	// blockchainBranchSize 为变长整数 <https://en.bitcoin.it/wiki/Protocol_documentation#Variable_length_integer> ，
	// 但是不太可能超过 0xFD。所以假设 blockchainBranchSize 只有一字节。
	blockchainBranchSize := int(data[index])
	index++

	// 读取 blockchain branch
	auxPowData.blockchainBranch.branchs = make([]hash.Byte32, blockchainBranchSize)
	for i := 0; i < blockchainBranchSize; i++ {
		copy(auxPowData.blockchainBranch.branchs[i][:], data[index:])
		index += 32
	}

	// 读取 blockchain branch 的 side mask
	sideMask = make([]byte, 4)
	copy(sideMask, data[index:])
	auxPowData.blockchainBranch.sideMask = binary.LittleEndian.Uint32(sideMask)
	index += 4

	// 验证最后是否只剩下80字节的区块头
	extraDataLen := len(data) - index - 80
	if extraDataLen != 0 {
		err = errors.New("AuxPowData has unexpected data: " + strconv.Itoa(extraDataLen) +
			" bytes between blockchainBranchSideMask and blockHeader")
		return
	}

	// 数据合法，解析完成
	return
}

// ExpandingBlockchainBranch 将特定币种的MerkleBranch添加到AuxPowData.blockchainBranch
func (auxPowData *AuxPowData) ExpandingBlockchainBranch(extBranch merkle.MerklePath) {
	branch := &auxPowData.blockchainBranch

	extBranchLen := uint(len(extBranch))
	branch.sideMask = branch.sideMask << extBranchLen

	extBranchItems := make([]hash.Byte32, extBranchLen)
	for i := uint(0); i < extBranchLen; i++ {
		extBranchItems[i] = extBranch[i].Hash
		if extBranch[i].UseFirstInConcatenation {
			branch.sideMask = branch.sideMask | (uint32(1) << i)
		}
	}

	branch.branchs = append(extBranchItems, branch.branchs...)
}

// ToBytes 把AuxPowData转换为字节流
func (auxPowData *AuxPowData) ToBytes() (data []byte) {

	// parent coinbase transaction
	data = append(data, auxPowData.coinbaseTxn...)

	// parent block hash
	data = append(data, auxPowData.blockHash[:]...)

	// parent coinbase branch
	data = append(data, byte(len(auxPowData.coinbaseBranch.branchs)))
	for _, branch := range auxPowData.coinbaseBranch.branchs {
		data = append(data, branch[:]...)
	}
	sideMask := make([]byte, 4)
	binary.LittleEndian.PutUint32(sideMask, auxPowData.coinbaseBranch.sideMask)
	data = append(data, sideMask...)

	// merged mining blockchain branch
	data = append(data, byte(len(auxPowData.blockchainBranch.branchs)))
	for _, branch := range auxPowData.blockchainBranch.branchs {
		data = append(data, branch[:]...)
	}
	sideMask = make([]byte, 4)
	binary.LittleEndian.PutUint32(sideMask, auxPowData.blockchainBranch.sideMask)
	data = append(data, sideMask...)

	// parent block header
	data = append(data, auxPowData.parentBlock...)

	return
}

// ToHex 把AuxPowData转换为十六进制字符串
func (auxPowData *AuxPowData) ToHex() string {
	return hex.EncodeToString(auxPowData.ToBytes())
}
