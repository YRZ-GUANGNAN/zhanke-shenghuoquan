// Package admin is business domain F: management, reports, review and statistics.
package admin

import (
	"campus-circle/backend/internal/module/admin/review"
	"time"
)

type Report struct {
	ID              uint          `json:"id" gorm:"primaryKey"`
	ReporterID      uint          `json:"reporterId" gorm:"index;uniqueIndex:idx_reports_reporter_request,priority:1"`
	ClientRequestID *string       `json:"-" gorm:"column:client_request_id;size:96;uniqueIndex:idx_reports_reporter_request,priority:2"`
	TargetType      string        `json:"targetType" gorm:"size:32;index"`
	TargetID        uint          `json:"targetId" gorm:"index"`
	Reason          string        `json:"reason" gorm:"size:240"`
	Detail          string        `json:"detail" gorm:"type:text"`
	ReviewStatus    review.Status `json:"reviewStatus" gorm:"size:24;index;default:submitted"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}
