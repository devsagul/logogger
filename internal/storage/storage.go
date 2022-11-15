// Package storage defines storage interface
package storage

import (
	"context"

	"logogger/internal/schema"
)

type MetricsStorage interface {
	Put(ctx context.Context, value schema.Metrics) error
	Extract(ctx context.Context, req schema.Metrics) (schema.Metrics, error)
	Increment(ctx context.Context, req schema.Metrics, value int64) error
	List(ctx context.Context) ([]schema.Metrics, error)
	BulkPut(ctx context.Context, values []schema.Metrics) error
	BulkUpdate(ctx context.Context, counters []schema.Metrics, gauges []schema.Metrics) error
	Ping(ctx context.Context) error
	Close() error
}
