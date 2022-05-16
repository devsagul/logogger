package poller

import (
	"logogger/internal/schema"
	"logogger/internal/storage"
	"math/rand"
	"reflect"
	"runtime"
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
		return nil, nil
	}

	r := schema.NewGauge("RandomValue", rand.Float64())
	err = p.store.Put(r)
	if err != nil {
		return nil, nil
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	reflected := reflect.ValueOf(memStats)
	for _, stat := range SysMetrics {
		f := reflected.FieldByName(stat).Interface()
		v, ok := f.(float64)
		var g schema.Metrics
		if ok {
			g = schema.NewGauge(stat, v)
		} else {
			v, _ := f.(int64)
			g = schema.NewGauge(stat, float64(v))
		}
		err := p.store.Put(g)
		if err != nil {
			return nil, nil
		}
	}

	return p.store.List()
}

func (p Poller) Reset() error {
	return p.store.Put(schema.NewCounter("PollCount", p.start))
}
