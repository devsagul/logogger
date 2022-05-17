package dumper

import "logogger/internal/schema"

type NoOpDumper struct{}

func (NoOpDumper) Dump([]schema.Metrics) error {
	return nil
}

func (NoOpDumper) Close() error {
	return nil
}
