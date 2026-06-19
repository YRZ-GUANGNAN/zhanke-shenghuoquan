// Package review is domain F's single review workflow used by all displayable content.
package review

import (
	"context"
	"time"
)

type Status string

const (
	StatusSubmitted Status = "submitted"
	StatusReviewing Status = "reviewing"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
)

type Task struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	ObjectType  string    `json:"objectType" gorm:"size:32;index:idx_review_object,unique"`
	ObjectID    uint      `json:"objectId" gorm:"index:idx_review_object,unique"`
	SubmitterID uint      `json:"submitterId" gorm:"index"`
	Risk        string    `json:"risk" gorm:"size:240"`
	Status      Status    `json:"status" gorm:"size:24;index;default:submitted"`
	Reason      string    `json:"reason" gorm:"size:240"`
	OperatorID  uint      `json:"operatorId" gorm:"index"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Submission struct {
	ObjectType  string
	ObjectID    uint
	SubmitterID uint
	Risk        string
}

func (Task) TableName() string { return "review_tasks" }

type Target interface {
	ObjectType() string
	SetReviewStatus(ctx context.Context, objectID uint, status Status) error
}
