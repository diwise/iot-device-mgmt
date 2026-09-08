package access

import (
	"testing"

	"github.com/matryer/is"
)

// DM-002: locks the exact allow/deny behavior shared by application
// logic and presentation enforcement. OPA result-model handling
// (legacy tenants vs access object) is covered in the auth package;
// this test pins the check both models feed into.
func TestIsAllowed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		allowed []string
		s       string
		want    bool
	}{
		{"exact match allowed", []string{"a", "b"}, "a", true},
		{"second entry allowed", []string{"a", "b"}, "b", true},
		{"unknown denied", []string{"a", "b"}, "c", false},
		{"empty always allowed", []string{"a"}, "", true},
		{"empty allowed against empty list", nil, "", true},
		{"unknown denied against empty list", nil, "a", false},
		{"case sensitive", []string{"Default"}, "default", false},
		{"no substring match", []string{"default-extra"}, "default", false},
		{"no whitespace trimming", []string{"a"}, " a", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			is.Equal(IsAllowed(tc.allowed, tc.s), tc.want)
		})
	}
}
