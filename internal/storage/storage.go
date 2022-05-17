package storage

import (
	"logogger/internal/schema"
)

type MetricsStorage interface {
	Put(value schema.Metrics) error
	Extract(req schema.Metrics) (schema.Metrics, error)
	Increment(req schema.Metrics, value int64) error
	List() ([]schema.Metrics, error)
	BulkPut(values []schema.Metrics) error
}
