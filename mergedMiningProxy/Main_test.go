package main

import (
	"merkle-tree-and-bitcoin/hash"
	"merkle-tree-and-bitcoin/merkle"
	"testing"
)

// 测试 bash.Hash() 和 hash.Byte32.Reverse()
func TestHashAndReverse(t *testing.T) {
	h := hash.Hash([]byte("A"))
	hReverse := h.Reverse()

	expectH := "1cd6ef71e6e0ff46ad2609d403dc3fee244417089aa4461245a4e4fe23a55e42"
	expectHReverse := "425ea523fee4a4451246a49a08174424ee3fdc03d40926ad46ffe0e671efd61c"

	if h.Hex() != expectH {
		t.Errorf("h.Hex() expected: %s, got: %s", expectH, h.Hex())
	}

	if h.HexReverse() != expectHReverse {
		t.Errorf("h.HeHexReversex() expected: %s, got: %s", expectHReverse, h.HexReverse())
	}

	if hReverse.Hex() != expectHReverse {
		t.Errorf("hReverse.Hex() expected: %s, got: %s", expectH, hReverse.Hex())
	}

	if hReverse.HexReverse() != expectH {
		t.Errorf("hReverse.HeHexReversex() expected: %s, got: %s", expectHReverse, hReverse.HexReverse())
	}
}

// 测试只有一个叶结点（叶结点就是 merkle root）的 Merkle Tree
func TestMerkleOnlyOneLeaf(t *testing.T) {
	h := hash.Hash([]byte{1, 2, 5, 8, 0})
	expectedH := "7b1dbc5469e8b55814186f0ee470af64543eafd39ebb26c6be0bc0140f5cd16f"

	if h.Hex() != expectedH {
		t.Fatalf("hash failed, expected: %s, got: %s", expectedH, h)
	}

	// 只有一个叶结点的 merkle tree
	tree := merkle.NewMerkleTree(merkle.Row{h})
	root := tree.MerkleRoot()
	pathLen := len(tree.MerklePathForLeaf(0))
	expectedPathLen := 0

	if root != h {
		t.Fatalf("tree.MerkleRoot() failed, expected: %s, got: %s", h.Hex(), root.Hex())
	}

	if pathLen != expectedPathLen {
		t.Fatalf("len(tree.MerklePathForLeaf(0))s failed, expected: %d, got: %d", expectedPathLen, pathLen)
	}
}
