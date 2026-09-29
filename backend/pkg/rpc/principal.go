package rpc

// principal.go
// Calling principal (Principal) definition. Distinguish between different subject types (anonymous/agent/user/API Key),
// Replacing the original single group string makes the identity information more structured and facilitates subsequent capability model expansion.

// PrincipalType calling principal type
type PrincipalType int

const (
	// PrincipalAnonymous anonymous visitor (unauthenticated)
	PrincipalAnonymous PrincipalType = iota
	// PrincipalAgent The agent client authenticated by client token
	PrincipalAgent
	// PrincipalUser Administrator user authenticated via session cookie
	PrincipalUser
	// PrincipalAPIKey The caller authenticated by the API Key
	PrincipalAPIKey
)

// Principal calls the subject, carrying identity information and capabilities.
type Principal struct {
	// Type subject type
	Type PrincipalType
	// UserUUID User UUID (exists when PrincipalUser)
	UserUUID string
	// ClientUUID agent client UUID (exists when PrincipalAgent)
	ClientUUID string
	// Whether IsAPIKey is an API Key call (quick determination, equivalent to Type==PrincipalAPIKey)
	IsAPIKey bool
	// Roles role/ability set. Default is deduced from Type:
	//   - PrincipalAnonymous → [RoleGuest]
	//   - PrincipalAgent → [RoleClient]
	//   - PrincipalUser / PrincipalAPIKey → [RoleAdmin]
	// In the future, it can be expanded to multiple roles (read-only admin / API Key scope, etc.).
	Roles []string
}

// NewAnonymousPrincipal creates an anonymous visitor principal
func NewAnonymousPrincipal() *Principal {
	return &Principal{
		Type:  PrincipalAnonymous,
		Roles: []string{RoleGuest},
	}
}

// NewAgentPrincipal creates agent client principal
func NewAgentPrincipal(clientUUID string) *Principal {
	return &Principal{
		Type:       PrincipalAgent,
		ClientUUID: clientUUID,
		Roles:      []string{RoleClient},
	}
}

// NewUserPrincipal creates an administrator user principal
func NewUserPrincipal(userUUID string) *Principal {
	return &Principal{
		Type:     PrincipalUser,
		UserUUID: userUUID,
		Roles:    []string{RoleAdmin},
	}
}

// NewAPIKeyPrincipal creates API Key calling body
func NewAPIKeyPrincipal() *Principal {
	return &Principal{
		Type:     PrincipalAPIKey,
		IsAPIKey: true,
		Roles:    []string{RoleAdmin},
	}
}

// PrimaryRole Returns the principal's primary role (compatible with existing single-role models).
// In a multi-role scenario, return the one with the highest authority level.
func (p *Principal) PrimaryRole() string {
	if p == nil || len(p.Roles) == 0 {
		return RoleGuest
	}
	bestRole := p.Roles[0]
	bestLevel := levelOf(bestRole)
	for _, r := range p.Roles[1:] {
		if lv := levelOf(r); lv > bestLevel {
			bestLevel = lv
			bestRole = r
		}
	}
	return bestRole
}

// HasRole determines whether the subject has the specified role
func (p *Principal) HasRole(role string) bool {
	if p == nil {
		return false
	}
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// PrincipalFromRole constructs a minimal principal based on role for internal calls (OnInternalRequest), etc.
// A scene in which only the characters are known, with no specific identifying information. Type is reasonably inferred based on role:
//
//	guest → Anonymous, client → Agent, admin → User.
//
// Note: This structure does not carry UUID/token, it is only used for permission determination and disclosure, and should not be used to audit actor ownership.
func PrincipalFromRole(role string) *Principal {
	switch role {
	case RoleClient:
		return &Principal{Type: PrincipalAgent, Roles: []string{RoleClient}}
	case RoleAdmin:
		return &Principal{Type: PrincipalUser, Roles: []string{RoleAdmin}}
	default:
		return NewAnonymousPrincipal()
	}
}
