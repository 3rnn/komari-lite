package rpc

import "testing"

func TestDestructiveAndCredentialRPCMethodsRequireSensitive2FA(t *testing.T) {
	for _, method := range []string{
		"admin:rotateClientToken",
		"admin:editSettings",
		"admin:updateAccountPreferences",
		"admin:deleteAllSessions",
		"admin:removeClient",
		"admin:clearRecords",
		"admin:clearAllRecords",
	} {
		if !IsSensitive(method) {
			t.Fatalf("%s must require sensitive 2FA", method)
		}
	}
}
