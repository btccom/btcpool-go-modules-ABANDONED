package main

import "errors"

// StratumError Stratum错误
type StratumError struct {
	// 错误号
	ErrNo int
	// 错误信息
	ErrMsg string
}

// NewStratumError 新建一个StratumError
func NewStratumError(errNo int, errMsg string) *StratumError {
	err := new(StratumError)
	err.ErrNo = errNo
	err.ErrMsg = errMsg

	return err
}

// Error 实现StratumError的Error()接口以便其被当做error类型使用
func (err *StratumError) Error() string {
	return err.ErrMsg
}

// ToJSONRPCArray 转换为JSONRPCArray
func (err *StratumError) ToJSONRPCArray() JSONRPCArray {
	if err == nil {
		return nil
	}

	return JSONRPCArray{err.ErrNo, err.ErrMsg, nil}
}

var (
	// ErrBufIOReadTimeout 从bufio.Reader中读取数据时超时
	ErrBufIOReadTimeout = errors.New("BufIO Read Timeout")
	// ErrSessionIDFull SessionID已满（所有可用值均已分配）
	ErrSessionIDFull = errors.New("Session ID is Full")
	// ErrParseSubscribeResponseFailed 解析订阅响应失败
	ErrParseSubscribeResponseFailed = errors.New("Parse Subscribe Response Failed")
	// ErrSessionIDInconformity 返回的会话ID和当前保存的不匹配
	ErrSessionIDInconformity = errors.New("Session ID Inconformity")
	// ErrAuthorizeFailed 认证失败
	ErrAuthorizeFailed = errors.New("Authorize Failed")
)

var (
	// StratumErrNeedSubscribed 需要订阅
	StratumErrNeedSubscribed = NewStratumError(101, "Need Subscribed")
	// StratumErrNeedAuthorize 需要认证
	StratumErrNeedAuthorize = NewStratumError(102, "Need Authorize")
	// StratumErrTooFewParams 参数太少
	StratumErrTooFewParams = NewStratumError(103, "Too Few Params")
	// StratumErrWorkerNameMustBeString 矿工名必须是字符串
	StratumErrWorkerNameMustBeString = NewStratumError(104, "Worker Name Must be a String")
	// StratumErrWorkerNameStartWrong 矿工名开头错误
	StratumErrWorkerNameStartWrong = NewStratumError(105, "Worker Name Cannot Start with '.'")

	// StratumErrStratumServerNotFound 找不到对应币种的Stratum Server
	StratumErrStratumServerNotFound = NewStratumError(301, "Stratum Server Not Found")
	// StratumErrConnectStratumServerFailed 对应币种的Stratum Server连接失败
	StratumErrConnectStratumServerFailed = NewStratumError(302, "Connect Stratum Server Failed")
)
