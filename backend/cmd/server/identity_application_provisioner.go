package main

import (
	"context"
	"errors"
	"time"

	"campus-circle/backend/internal/module/identityapplication"
	"campus-circle/backend/internal/module/takeout"
	"gorm.io/gorm"
)

func newIdentityApplicationApprovalProvisioner(now func() time.Time) identityapplication.ApprovalProvisioner {
	return identityapplication.ApprovalProvisionerFunc(func(
		ctx context.Context,
		tx *gorm.DB,
		approved identityapplication.ApprovedIdentity,
	) error {
		if approved.Role != identityapplication.RoleRider {
			return identityapplication.ErrInvalidRole
		}
		if tx == nil {
			return errors.New("identity application approval transaction is required")
		}
		riders, err := takeout.NewRiderIdentityService(takeout.NewRepository(tx), now)
		if err != nil {
			return err
		}
		_, err = riders.RegisterApproved(ctx, approved.UserID, approved.RealName, approved.Phone)
		if errors.Is(err, takeout.ErrRiderUnavailable) {
			return identityapplication.ErrDecisionConflict
		}
		return err
	})
}
