package main

import (
	"crypto/subtle"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"merkle-tree-and-bitcoin/hash"

	"github.com/golang/glog"
)

// RPCResultCreateAuxBlock RPC方法createauxblock的返回结果
type RPCResultCreateAuxBlock struct {
	Hash          string `json:"hash"`
	ChainID       uint32 `json:"chainid"`
	PrevBlockHash string `json:"previousblockhash"`
	CoinbaseValue uint64 `json:"coinbasevalue"`
	Bits          string `json:"bits"`
	Height        uint32 `json:"height"`
	Target        string `json:"_target"`
	MerkleSize    uint32 `json:"merkle_size"`
	MerkleNonce   uint32 `json:"merkle_nonce"`
}

// write 输出JSON-RPC格式的信息
func write(w http.ResponseWriter, response interface{}) {
	responseJSON, _ := json.Marshal(response)
	w.Write(responseJSON)
}

// writeError 输出JSON-RPC格式的错误信息
func writeError(w http.ResponseWriter, id interface{}, errNo int, errMsg string) {
	err := RPCError{errNo, errMsg}
	response := RPCResponse{id, nil, err}
	write(w, response)
}

// ProxyRPCHandle 代理RPC处理器
type ProxyRPCHandle struct {
	config      ProxyRPCServer
	auxJobMaker *AuxJobMaker
	dbhandle    DBConnection
}

// NewProxyRPCHandle 创建代理RPC处理器
func NewProxyRPCHandle(config ProxyRPCServer, auxJobMaker *AuxJobMaker) (handle *ProxyRPCHandle) {
	handle = new(ProxyRPCHandle)
	handle.config = config
	handle.auxJobMaker = auxJobMaker
	handle.dbhandle.InitDB(config.PoolDb)
	return
}

// basicAuth 执行Basic认证
func (handle *ProxyRPCHandle) basicAuth(r *http.Request) bool {
	apiUser := []byte(handle.config.User)
	apiPasswd := []byte(handle.config.Passwd)

	user, passwd, ok := r.BasicAuth()

	// 检查用户名密码是否正确
	if ok && subtle.ConstantTimeCompare(apiUser, []byte(user)) == 1 && subtle.ConstantTimeCompare(apiPasswd, []byte(passwd)) == 1 {
		return true
	}

	return false
}

func (handle *ProxyRPCHandle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !handle.basicAuth(r) {
		// 认证失败，提示 401 Unauthorized
		// Restricted 可以改成其他的值
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		// 401 状态码
		w.WriteHeader(http.StatusUnauthorized)
		// 401 页面
		w.Write([]byte(`<h1>401 - Unauthorized</h1>`))
		return
	}

	if r.Method != "POST" {
		w.Write([]byte("JSONRPC server handles only POST requests"))
		return
	}

	requestJSON, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeError(w, nil, 400, err.Error())
		return
	}

	var request RPCRequest
	err = json.Unmarshal(requestJSON, &request)
	if err != nil {
		writeError(w, nil, 400, err.Error())
		return
	}

	response := RPCResponse{request.ID, nil, nil}

	switch request.Method {
	case "createauxblock":
		handle.createAuxBlock(&response)
	case "submitauxblock":
		handle.submitAuxBlock(request.Params, &response)
	case "getauxblock":
		if len(request.Params) > 0 {
			handle.submitAuxBlock(request.Params, &response)
		} else {
			handle.createAuxBlock(&response)
		}
	default:
		// 将未知方法转发给第一个chain的server
		responseJSON, err := RPCCall(handle.auxJobMaker.chains[0].RPCServer, request.Method, request.Params)
		if err != nil {
			writeError(w, nil, 400, err.Error())
			return
		}
		response, err = ParseRPCResponse(responseJSON)
		if err != nil {
			writeError(w, nil, 400, err.Error())
			return
		}
		// 若调用的是help方法，则在结果后面追加对 createauxblock 和 submitauxblock 的描述
		if request.Method == "help" && len(request.Params) == 0 {
			helpStr, ok := response.Result.(string)
			if ok {
				helpStr += "\n\n== Merged Mining Proxy ==\n" +
					"createauxblock <address>\n" +
					"submitauxblock <hash> <auxpow>\n" +
					"getauxblock (hash auxpow)"
				response.Result = helpStr
			}
		}
	}

	write(w, response)
}

func (handle *ProxyRPCHandle) createAuxBlock(response *RPCResponse) {
	job, err := handle.auxJobMaker.GetAuxJob()
	if err != nil {
		response.Error = RPCError{500, err.Error()}
		return
	}

	var result RPCResultCreateAuxBlock
	result.Bits = job.MinBits
	result.ChainID = 1
	result.CoinbaseValue = job.CoinbaseValue
	result.Hash = job.MerkleRoot.HexReverse()
	result.Height = job.Height
	result.PrevBlockHash = job.PrevBlockHash.Hex()
	result.Target = job.MaxTarget.HexReverse()
	result.MerkleSize = job.MerkleSize
	result.MerkleNonce = job.MerkleNonce

	glog.Info("[CreateAuxBlock] height:", result.Height,
		", bits:", result.Bits,
		", target:", job.MaxTarget.Hex(),
		", coinbaseValue:", result.CoinbaseValue,
		", hash:", job.MerkleRoot.Hex(),
		", prevHash:", result.PrevBlockHash,
		", merkleSize: ", job.MerkleSize,
		", merkleNonce: ", job.MerkleNonce)

	response.Result = result
	return
}

func (handle *ProxyRPCHandle) submitAuxBlock(params []interface{}, response *RPCResponse) {
	if len(params) < 2 {
		response.Error = RPCError{400, "The number of params should be 2"}
		return
	}

	hashHex, ok := params[0].(string)
	if !ok {
		response.Error = RPCError{400, "The param 1 should be a string"}
		return
	}

	auxPowHex, ok := params[1].(string)
	if !ok {
		response.Error = RPCError{400, "The param 2 should be a string"}
		return
	}

	auxPowData, err := ParseAuxPowData(auxPowHex, handle.config.MainChain)
	if err != nil {
		response.Error = RPCError{400, err.Error()}
		return
	}

	hashtmp, err := hash.MakeByte32FromHex(hashHex)
	if err != nil {
		response.Error = RPCError{400, err.Error()}
		return
	}
	hashtmp = hashtmp.Reverse()

	job, err := handle.auxJobMaker.FindAuxJob(hashtmp)
	if err != nil {
		response.Error = RPCError{400, err.Error()}
		return
	}

	count := 0
	for index, extAuxPow := range job.AuxPows {
		if glog.V(3) {
			glog.Info("[SubmitAuxBlock] <", handle.auxJobMaker.chains[index].Name, "> blockHash: ",
				auxPowData.blockHash.Hex(), "; auxTarget: ", extAuxPow.Target.Hex())
		}

		// target reached
		if auxPowData.blockHash.Hex() <= extAuxPow.Target.Hex() {

			go func(index int, auxPowData AuxPowData, extAuxPow AuxPowInfo) {
				chain := handle.auxJobMaker.chains[index]
				auxPowData.ExpandingBlockchainBranch(extAuxPow.BlockchainBranch)
				auxPowHex := auxPowData.ToHex()

				// 切片是对原字符串的引用
				// 对切片中字符串的修改会直接改变 chain.SubmitAuxBlock.Params 中的值
				// 所以这里拷贝一份

				params := DeepCopy(chain.SubmitAuxBlock.Params)

				if paramsArr, ok := params.([]interface{}); ok { // JSON-RPC 1.0 param array
					for i := range paramsArr {
						if str, ok := paramsArr[i].(string); ok {
							str = strings.Replace(str, "{hash-hex}", extAuxPow.Hash.HexReverse(), -1)
							str = strings.Replace(str, "{aux-pow-hex}", auxPowHex, -1)
							paramsArr[i] = str
						}
					}

				} else if paramsMap, ok := params.(map[string]interface{}); ok { // JSON-RPC 2.0 param object
					for k := range paramsMap {
						if str, ok := paramsMap[k].(string); ok {
							str = strings.Replace(str, "{hash-hex}", extAuxPow.Hash.HexReverse(), -1)
							str = strings.Replace(str, "{aux-pow-hex}", auxPowHex, -1)
							paramsMap[k] = str
						}
					}
				}

				response, err := RPCCall(chain.RPCServer, chain.SubmitAuxBlock.Method, params)

				{
					var submitauxblockinfo SubmitAuxBlockInfo
					submitauxblockinfo.AuxBlockTableName = handle.auxJobMaker.chains[index].AuxTableName
					if handle.config.MainChain == "LTC" {
						submitauxblockinfo.ParentChainBllockHash = HexToString(ArrayReverse(DoubleSHA256(auxPowData.parentBlock)))
					} else {
						submitauxblockinfo.ParentChainBllockHash = auxPowData.blockHash.Hex()
					}

					submitauxblockinfo.AuxChainBlockHash = extAuxPow.Hash.Hex()
					submitauxblockinfo.AuxPow = auxPowHex
					submitauxblockinfo.CurrentTime = time.Now().Format("2006-01-02 15:04:05") 

					if ok = handle.dbhandle.InsertAuxBlock(submitauxblockinfo); !ok {
						glog.Warning("Insert AuxBlock to db failed!")
					}

					glog.Info(
						"[SubmitAuxBlock] <", handle.auxJobMaker.chains[index].Name, "> ",
						", height: ", extAuxPow.Height,
						", hash: ", extAuxPow.Hash.Hex(),
						", parentBlockHash: ", submitauxblockinfo.ParentChainBllockHash,
						", target: ", extAuxPow.Target.Hex(),
						", response: ", string(response),
						", errmsg: ", err)
				}

			}(index, *auxPowData, extAuxPow)

			count++
		}
	}

	if count < 1 {
		glog.Warning("[SubmitAuxBlock] high hash! blockHash: ", auxPowData.blockHash.Hex(), "; maxTarget: ", job.MaxTarget.Hex())
		response.Error = RPCError{400, "high-hash"}
		return
	}

	response.Result = true
	return
}

func runHTTPServer(config ProxyRPCServer, auxJobMaker *AuxJobMaker) {
	handle := NewProxyRPCHandle(config, auxJobMaker)

	// HTTP监听
	glog.Info("Listen HTTP ", config.ListenAddr)
	err := http.ListenAndServe(config.ListenAddr, handle)

	if err != nil {
		glog.Fatal("HTTP Listen Failed: ", err)
		return
	}
}
