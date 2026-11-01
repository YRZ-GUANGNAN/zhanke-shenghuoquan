package notification

import (
	"context"
	"time"
)

type Consumer struct {
	Expander *Expander
	Interval time.Duration
	Now      func() time.Time
}

func NewConsumer(expander *Expander) *Consumer {
	return &Consumer{Expander: expander, Interval: time.Second, Now: time.Now}
}

func (c *Consumer) RunOnce(ctx context.Context, now time.Time) (bool, error) {
	if c == nil || c.Expander == nil {
		return false, ErrInvalidRecord
	}
	return c.Expander.ExpandOnce(ctx, now)
}

func (c *Consumer) Run(ctx context.Context) error {
	if c == nil || c.Expander == nil || c.Interval <= 0 {
		return ErrInvalidRecord
	}
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			now := time.Now()
			if c.Now != nil {
				now = c.Now()
			}
			if _, err := c.RunOnce(ctx, now); err != nil {
				// A single malformed event is persisted as retry/deadletter by the
				// expander; keep the worker alive for later events.
				continue
			}
		}
	}
}
