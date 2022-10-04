// Package dumper defines interface for data dumpers
package dumper

import "logogger/internal/schema"

type Dumper interface {
	Dump(values []schema.Metrics) error
	Close() error
}
