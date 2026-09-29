package rpc

// permission.go
// Declarative ACL permissions model. Permissions are declared by a set of rules (pattern, minRole), pattern supports the "*" wildcard character,
// Matches the full method name (such as "admin:addClient", "rpc.ping"). When judging rights, take it from all matching rules.
// Whichever has the highest specificity: exact match > wildcard with longer literal prefix > global "*".
//
// The model is extensible for plugins: plugins can declare rules at any granularity (namespace level "ns:*" or method level) using Allow,
// No need to modify the core code.
//
// Roles adopt hierarchical semantics: guest < client < admin, and the rules declare the minimum required roles.

import (
	"strings"
	"sync"
)

// Role constants. Consistent with roles in web/api (guest/client/admin).
const (
	RoleGuest  = "guest"
	RoleClient = "client"
	RoleAdmin  = "admin"
)

// roleLevel defines the permission level of the role. The larger the value, the higher the authority. Unknown roles are handled as guest (minimum).
var roleLevel = map[string]int{
	RoleGuest:  0,
	RoleClient: 1,
	RoleAdmin:  2,
}

// DefaultNamespace is the namespace into which the method name does not contain ":".
const DefaultNamespace = "common"

// aclRule A declarative ACL rule.
type aclRule struct {
	pattern string // Method name matching pattern, supports "*" wildcard
	minRole string // Minimum role required to match
	// Precomputed specificity: Exact matches (no wildcards) give bonus to high bits, and literal character count for the rest.
	specificity int
	hasWildcard bool
}

var (
	muACL   sync.RWMutex
	aclList []aclRule
)

func init() {
	// Default rules override built-in namespace semantics. "*" Requires admin to ensure that undeclared methods default to the strictest.
	Allow("*", RoleAdmin)
	Allow("common:*", RoleGuest)
	Allow("guest:*", RoleGuest)
	Allow("rpc.*", RoleGuest) // Internal methods rpc.ping/rpc.help etc. (separated by ".")
	Allow("rpc:*", RoleGuest)
	Allow("client:*", RoleClient)
	Allow("admin:*", RoleAdmin)
}

// levelOf returns the permission level of the role. Unknown roles are considered guests.
func levelOf(role string) int {
	if lv, ok := roleLevel[role]; ok {
		return lv
	}
	return roleLevel[RoleGuest]
}

// NamespaceOf resolves the namespace of method names. "ns:method" returns "ns"; without ":" returns DefaultNamespace.
func NamespaceOf(method string) string {
	if i := strings.IndexByte(method, ':'); i >= 0 {
		return method[:i]
	}
	return DefaultNamespace
}

// computeSpecificity computes the specificity of pattern. Exact matches (without wildcards) give a big bonus
// To ensure the highest priority; those containing wildcards are sorted by the literal number of characters (not "*"), and the longer the prefix, the more specific it is.
func computeSpecificity(pattern string) (spec int, hasWildcard bool) {
	literal := 0
	for _, r := range pattern {
		if r != '*' {
			literal++
		} else {
			hasWildcard = true
		}
	}
	if !hasWildcard {
		return 1 << 20, false // Exact match highest priority
	}
	return literal, true
}

// Allow declares an ACL rule: methods matching pattern allow roles of minRole and above to be called.
// Repeated declarations of the same pattern will overwrite the original rules. For plugins to declare custom permissions.
func Allow(pattern, minRole string) {
	spec, hasWildcard := computeSpecificity(pattern)
	rule := aclRule{pattern: pattern, minRole: minRole, specificity: spec, hasWildcard: hasWildcard}
	muACL.Lock()
	defer muACL.Unlock()
	for i := range aclList {
		if aclList[i].pattern == pattern {
			aclList[i] = rule
			return
		}
	}
	aclList = append(aclList, rule)
}

// RegisterNamespace convenience wrapper: declares the minimum required role for the entire namespace (equivalent to Allow("ns:*", role)).
func RegisterNamespace(namespace, requiredRole string) {
	Allow(namespace+":*", requiredRole)
}

// wildcardMatch determines whether s matches the pattern containing only "*" wildcard characters. "*" matches any (including empty) sequence of characters.
func wildcardMatch(pattern, s string) bool {
	// Double pointers + backtracking, O(len(pattern)+len(s)).
	var (
		p, str       = 0, 0
		star         = -1
		strBackup    = 0
		lenP, lenStr = len(pattern), len(s)
	)
	for str < lenStr {
		if p < lenP && (pattern[p] == s[str]) {
			p++
			str++
		} else if p < lenP && pattern[p] == '*' {
			star = p
			strBackup = str
			p++
		} else if star != -1 {
			p = star + 1
			strBackup++
			str = strBackup
		} else {
			return false
		}
	}
	for p < lenP && pattern[p] == '*' {
		p++
	}
	return p == lenP
}

// resolveMinRole Returns the required role with the highest specificity among the applicable rules for method. If there is no matching rule, it defaults to admin.
func resolveMinRole(method string) string {
	// Normalization: Naked method names without namespace separators (neither ":" nor "rpc." internal prefix) are grouped into the default namespace.
	// In order to be matched by rules such as "common:*" to maintain consistency with historical behavior.
	if !strings.ContainsAny(method, ":") && !strings.HasPrefix(method, "rpc.") {
		method = DefaultNamespace + ":" + method
	}
	muACL.RLock()
	defer muACL.RUnlock()
	bestSpec := -1
	bestRole := RoleAdmin
	matched := false
	for i := range aclList {
		r := &aclList[i]
		if !wildcardMatch(r.pattern, method) {
			continue
		}
		// The one with higher specificity wins; with the same specificity, take the more stringent (higher level) role, favoring safety.
		if r.specificity > bestSpec || (r.specificity == bestSpec && levelOf(r.minRole) > levelOf(bestRole)) {
			bestSpec = r.specificity
			bestRole = r.minRole
			matched = true
		}
	}
	if !matched {
		return RoleAdmin
	}
	return bestRole
}

// RequiredRole Returns the minimum role required to call method.
func RequiredRole(method string) string {
	return resolveMinRole(method)
}

// CheckPermission determines whether the group role has the right to call the method.
func CheckPermission(group, method string) bool {
	return CheckPrincipal(PrincipalFromRole(group), method)
}

// CheckPrincipal determines whether the principal has the right to call the method based on the principal's capability set.
// Use set membership semantics (rather than linear hierarchy): the method requires a certain minimum role that the subject must hold in its role set;
// guest is a public baseline and is implicitly accessible to any principal.
// This model makes agent and admin orthogonal subjects: admin no longer automatically obtains client capabilities (and vice versa),
// This blocks unauthorized paths such as "the admin session impersonates the agent and calls the client:* reporting method".
func CheckPrincipal(p *Principal, method string) bool {
	if p == nil {
		p = NewAnonymousPrincipal()
	}
	min := resolveMinRole(method)
	if min == RoleGuest {
		return true // Public baseline, accessible to all subjects
	}
	return p.HasRole(min)
}
