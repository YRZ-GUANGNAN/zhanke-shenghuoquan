package deliveryim

import (
	"context"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/im"
)

type GroupAdapter struct {
	users  *im.Service
	groups im.WorkGroupManager
}

func NewGroupAdapter(users *im.Service, groups im.WorkGroupManager) (*GroupAdapter, error) {
	if users == nil || groups == nil {
		return nil, delivery.ErrInvalidRecord
	}
	return &GroupAdapter{users: users, groups: groups}, nil
}

func (a *GroupAdapter) EnsureWorkGroup(
	ctx context.Context,
	groupID, name string,
	merchantUserID, riderUserID uint64,
) error {
	merchant, err := a.users.EnsureUser(ctx, uint(merchantUserID))
	if err != nil {
		return err
	}
	rider, err := a.users.EnsureUser(ctx, uint(riderUserID))
	if err != nil {
		return err
	}
	// Both delivery participants must remain ordinary members. The group has no
	// business-user owner so MuteAllMember can enforce terminal read-only mode
	// for both merchant and rider at Tencent IM, not only in local metadata.
	return a.groups.CreateWorkGroup(ctx, groupID, name, "", []string{merchant, rider})
}

func (a *GroupAdapter) RemoveWorkGroupMember(
	ctx context.Context,
	groupID string,
	riderUserID uint64,
) error {
	rider, err := a.users.EnsureUser(ctx, uint(riderUserID))
	if err != nil {
		return err
	}
	return a.groups.RemoveWorkGroupMembers(ctx, groupID, []string{rider})
}

func (a *GroupAdapter) DismissWorkGroup(ctx context.Context, groupID string) error {
	return a.groups.DismissWorkGroup(ctx, groupID)
}

func (a *GroupAdapter) SetWorkGroupReadOnly(
	ctx context.Context,
	groupID string,
	readOnly bool,
) error {
	return a.groups.SetWorkGroupReadOnly(ctx, groupID, readOnly)
}

var _ delivery.ConversationGroupPort = (*GroupAdapter)(nil)
