package jsonrpc

import "testing"

// TestPrivateSiteLoginWhitelist guards issue #567: login-page metadata must remain accessible
// in private-site mode, or guests cannot see the login form.
func TestPrivateSiteLoginWhitelist(t *testing.T) {
	required := []string{
		"public:getMe",
		"public:getPublicSettings",
		"public:getVersion",
		"public:recordVisitorEvent",
	}
	for _, m := range required {
		if !privateSiteLoginWhitelist[m] {
			t.Errorf("method %q must be in privateSiteLoginWhitelist for login page to render under private site (issue #567)", m)
		}
	}

	// Data interfaces such as node lists should not be whitelisted (should be blocked by private sites).
	mustBlocked := []string{
		"public:getNodesInformation",
		"public:getRecordsByUUID",
		"public:getPingRecords",
	}
	for _, m := range mustBlocked {
		if privateSiteLoginWhitelist[m] {
			t.Errorf("data method %q must NOT be in privateSiteLoginWhitelist (would leak data under private site)", m)
		}
	}
}
