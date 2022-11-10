// Package storage defines storage interface
package storage

import (
	"context"

	"logogger/internal/schema"
)

type MetricsStorage interface {
	Put(value schema.Metrics) error
	Extract(req schema.Metrics) (schema.Metrics, error)
	Increment(req schema.Metrics, value int64) error
	List() ([]schema.Metrics, error)
	BulkPut(values []schema.Metrics) error
	BulkUpdate(counters []schema.Metrics, gauges []schema.Metrics) error
	Ping() error
	Close() error
	WithContext(ctx context.Context) MetricsStorage
}
