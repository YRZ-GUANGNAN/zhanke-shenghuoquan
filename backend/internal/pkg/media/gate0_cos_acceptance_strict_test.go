package media

import (
	"net/http"
	"testing"
)

// gate0V11RequireRejectedStrict prevents infrastructure failures such as
// NoSuchBucket from being misclassified as a successful policy rejection.
func gate0V11RequireRejectedPolicy(t *testing.T, label string, status int, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s did not receive a COS response: %v", label, err)
	}
	if status != http.StatusForbidden {
		t.Fatalf("%s was not rejected by COS authentication/policy: status=%d", label, status)
	}
	t.Logf("PASS %s rejected by COS authentication/policy: status=%d", label, status)
}
