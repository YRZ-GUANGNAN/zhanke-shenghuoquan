// Package dbtx carries a GORM transaction through a request context without
// making business modules depend on one another.
package dbtx

import (
	"context"

	"gorm.io/gorm"
)

type contextKey struct{}

// WithDB attaches db to ctx. Callers should only attach an active transaction.
func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, contextKey{}, db)
}

// FromContext returns the transaction attached to ctx, when one exists.
func FromContext(ctx context.Context) (*gorm.DB, bool) {
	db, ok := ctx.Value(contextKey{}).(*gorm.DB)
	return db, ok && db != nil
}

// DB joins a transaction already attached to ctx, otherwise it uses fallback.
func DB(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if db, ok := FromContext(ctx); ok {
		return db.WithContext(ctx)
	}
	return fallback.WithContext(ctx)
}
