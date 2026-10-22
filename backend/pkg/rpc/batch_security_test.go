package rpc

import (
	"strings"
	"testing"
)

func TestParseRequestsRejectsExcessiveBatch(t *testing.T) {
	request := `{"jsonrpc":"2.0","id":1,"method":"public:getVersion"}`
	data := []byte("[" + strings.Repeat(request+",", 64) + request + "]")
	if _, err := ParseRequests(data); err == nil {
		t.Fatal("accepted 65-call batch")
	}
	data = []byte("[" + strings.Repeat(request+",", 63) + request + "]")
	if requests, err := ParseRequests(data); err != nil || len(requests) != 64 {
		t.Fatal("valid 64-call batch rejected")
	}
}
