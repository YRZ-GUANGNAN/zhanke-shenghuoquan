package media

import (
	"context"
)

// Controller is an in-process application boundary, not an HTTP controller.
// Transport registration is intentionally deferred beyond Module 2.
type Controller struct{ service *Service }

func NewController(service *Service) *Controller { return &Controller{service: service} }

func (c *Controller) CreateAssetRecord(ctx context.Context, input NewAsset) (Asset, error) {
	return c.service.CreateAssetRecord(ctx, input)
}

func (c *Controller) FindAsset(ctx context.Context, publicID string) (Asset, error) {
	return c.service.FindAsset(ctx, publicID)
}

func (c *Controller) TransitionTechnical(ctx context.Context, id, version uint64, from, to TechnicalStatus) error {
	return c.service.TransitionTechnical(ctx, id, version, from, to)
}
