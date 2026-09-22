package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	v2 "github.com/nuomiiiii/lite-agent/protocol/v2"
)

type httpStatusError struct {
	StatusCode int
	Status     string
	Body       string
}

type v2ProtocolError struct {
	Err error
}

func (e *v2ProtocolError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *v2ProtocolError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *httpStatusError) Error() string {
	if e == nil {
		return ""
	}
	if e.Body != "" {
		return fmt.Sprintf("status code: %d,%s", e.StatusCode, e.Body)
	}
	if e.Status != "" {
		return e.Status
	}
	return fmt.Sprintf("status code: %d", e.StatusCode)
}

func isHTTPStatus(err error, statusCode int) bool {
	var statusErr *httpStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == statusCode
}

func newV2ProtocolError(err error) error {
	if err == nil {
		return nil
	}
	return &v2ProtocolError{Err: err}
}

func parseV2Response(body []byte) (*v2.Response, error) {
	var rpcResp v2.Response
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, newV2ProtocolError(fmt.Errorf("invalid v2 JSON-RPC response: %w, body: %s", err, bodySnippet(body)))
	}
	if rpcResp.JSONRPC != v2.Version {
		return nil, newV2ProtocolError(fmt.Errorf("invalid v2 JSON-RPC version %q, body: %s", rpcResp.JSONRPC, bodySnippet(body)))
	}
	if rpcResp.Error != nil {
		return &rpcResp, newV2ProtocolError(fmt.Errorf("v2 rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message))
	}
	return &rpcResp, nil
}

func bodySnippet(body []byte) string {
	const max = 120
	if len(body) > max {
		body = body[:max]
	}
	return fmt.Sprintf("%q", string(body))
}

// postV2RPC 向面板投递一条 v2 JSON-RPC 请求（原来的实现随远程执行子系统一起被删除，
// 这里保留通用传输能力：路由探测与基础信息上报仍需要它）。
func postV2RPC(payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	status, respBody, err := postV2JSONRPC(context.Background(), body, 30*time.Second)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return &httpStatusError{StatusCode: status, Status: http.StatusText(status), Body: string(respBody)}
	}
	return nil
}
