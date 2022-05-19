package poller

import (
	"fmt"
	"logogger/internal/schema"
	"logogger/internal/storage"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"

	"golang.org/x/sync/errgroup"
)

var SysMetrics = [...]string{
	"Alloc",
	"BuckHashSys",
	"Frees",
	"GCCPUFraction",
	"GCSys",
	"HeapAlloc",
	"HeapIdle",
	"HeapInuse",
	"HeapObjects",
	"HeapReleased",
	"HeapSys",
	"LastGC",
	"Lookups",
	"MCacheInuse",
	"MCacheSys",
	"MSpanInuse",
	"MSpanSys",
	"Mallocs",
	"NextGC",
	"NumForcedGC",
	"NumGC",
	"OtherSys",
	"PauseTotalNs",
	"StackInuse",
	"StackSys",
	"Sys",
	"TotalAlloc",
}

type Poller struct {
	store storage.MetricsStorage
	start int64
}

func NewPoller(start int64) (Poller, error) {
	store := storage.NewMemStorage()
	err := store.Put(schema.NewCounter("PollCount", start))
	return Poller{store, start}, err
}

func (p Poller) Poll() ([]schema.Metrics, error) {
	err := p.store.Increment(schema.NewCounterRequest("PollCount"), 1)
	if err != nil {
		return nil, err
	}

	r := schema.NewGauge("RandomValue", rand.Float64())
	err = p.store.Put(r)
	if err != nil {
		return nil, err
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	reflected := reflect.ValueOf(memStats)

	eg := &errgroup.Group{}
	for _, stat := range SysMetrics {
		stat := stat
		eg.Go(func() error {
			v := reflected.FieldByName(stat).Interface()
			f, err := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
			if err != nil {
				return err
			}
			err = p.store.Put(schema.NewGauge(stat, f))
			if err != nil {
				return err
			}
			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		return nil, err
	}
	return p.store.List()
}

func (p Poller) Reset() error {
	return p.store.Put(schema.NewCounter("PollCount", p.start))
}
