package main

import (
	"context"
	"errors"
	"testing"

	"campus-circle/backend/internal/module/identityapplication"
)

func TestIdentityApplicationApprovalProvisionerFailsClosed(t *testing.T) {
	provisioner := newIdentityApplicationApprovalProvisioner(nil)
	if provisioner == nil {
		t.Fatal("production rider approval provisioner is nil")
	}
	approved := identityapplication.ApprovedIdentity{
		UserID: 41, Role: identityapplication.RoleRider,
		RealName: "张同学", Phone: "13800138000",
	}
	if err := provisioner.ProvisionApproved(context.Background(), nil, approved); err == nil {
		t.Fatal("provisioner accepted a missing approval transaction")
	}
	approved.Role = identityapplication.RoleMerchant
	if err := provisioner.ProvisionApproved(context.Background(), nil, approved); !errors.Is(err, identityapplication.ErrInvalidRole) {
		t.Fatalf("merchant input err=%v", err)
	}
}
