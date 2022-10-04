package dumper

import (
	"encoding/json"
	"logogger/internal/schema"
	"os"
	"sync"
)

type SyncDumper struct {
	filename string
	wg       sync.WaitGroup
	mu       sync.Mutex
}

func (d *SyncDumper) Dump(l []schema.Metrics) error {
	d.wg.Add(1)
	defer d.wg.Done()

	b, err := json.Marshal(l)
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	err = os.WriteFile(d.filename, b, 0644)
	return err
}

func (d *SyncDumper) Close() error {
	d.wg.Wait()
	return nil
}

func NewSyncDumper(filename string) *SyncDumper {
	d := new(SyncDumper)
	d.filename = filename
	d.mu = sync.Mutex{}

	return d
}
