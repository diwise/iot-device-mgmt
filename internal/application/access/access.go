// Package access owns the generic tenant/access check used across the
// application layer. Presentation translates OPA policy results into
// the allowed list via auth.GetTenantsWithAllowedScopes and then
// enforces it here, as does application logic itself.
//
// It lives in its own leaf package (rather than on the application
// root) so both application subpackages and presentation can import it
// without creating an import cycle (DM-002).
package access

import "slices"

// IsAllowed reports whether s is within the allowed tenants. An empty
// s is always allowed, letting callers distinguish "no tenant to
// check" from a denied tenant.
func IsAllowed(allowedTenants []string, s string) bool {
	return slices.Contains(allowedTenants, s) || s == ""
}
