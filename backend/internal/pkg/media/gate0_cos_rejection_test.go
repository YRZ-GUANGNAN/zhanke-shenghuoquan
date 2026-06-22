package media

import (
	"strings"
	"testing"
)

func gate0V11RequireRejectedStrict(t *testing.T, label string, status int, err error) {
	t.Helper()
	if strings.Contains(label, "overwrite") {
		gate0V11RequireOverwriteConflict(t, label, status, err)
		return
	}
	gate0V11RequireRejectedPolicy(t, label, status, err)
}
