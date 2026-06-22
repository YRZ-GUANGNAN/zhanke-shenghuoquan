package media

import (
	"net/http"
	"testing"
)

func gate0V11RequireOverwriteConflict(t *testing.T, label string, status int, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s did not receive a COS response: %v", label, err)
	}
	if status != http.StatusConflict {
		t.Fatalf("%s was not rejected by forbid-overwrite: status=%d", label, status)
	}
	t.Logf("PASS %s rejected by forbid-overwrite: status=%d", label, status)
}
