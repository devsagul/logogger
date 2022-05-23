package dumper

import "logogger/internal/schema"

type Dumper interface {
	Dump(values []schema.Metrics) error
	Close() error
}
