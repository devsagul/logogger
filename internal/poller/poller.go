// Package poller implements metrics polling logic
package poller

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/sync/errgroup"

	"logogger/internal/schema"
	"logogger/internal/storage"
	"logogger/internal/utils"
)

const (
	pollCount   = "PollCount"
	randomValue = "RandomValue"
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

func NewPoller(ctx context.Context, start int64) (Poller, error) {
	store := storage.NewMemStorage()
	err := store.Put(ctx, schema.NewCounter(pollCount, start))
	return Poller{store, start}, err
}

func (p Poller) Poll(ctx context.Context) ([]schema.Metrics, error) {
	err := p.store.Increment(ctx, schema.NewCounterRequest(pollCount), 1)
	if err != nil {
		return nil, err
	}

	r := schema.NewGauge(randomValue, rand.Float64())
	err = p.store.Put(ctx, r)
	if err != nil {
		return nil, err
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	reflected := reflect.ValueOf(memStats)

	eg := &errgroup.Group{}

	for _, stat := range SysMetrics {
		stat := stat
		eg.Go(utils.WrapGoroutinePanic(func() error {
			v := reflected.FieldByName(stat).Interface()
			f, err_ := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
			if err_ != nil {
				return err_
			}
			err_ = p.store.Put(ctx, schema.NewGauge(stat, f))
			return err_
		}))
	}

	eg.Go(utils.WrapGoroutinePanic(func() error {
		v, err_ := mem.VirtualMemory()
		if err_ != nil {
			return err
		}

		err_ = p.store.Put(ctx, schema.NewGauge("TotalMemory", float64(v.Total)))
		if err_ != nil {
			return err_
		}

		err_ = p.store.Put(ctx, schema.NewGauge("FreeMemory", float64(v.Free)))
		return err_
	}))

	eg.Go(utils.WrapGoroutinePanic(func() error {
		utilization, err_ := cpu.Percent(0, true)
		if err_ != nil {
			return err_
		}

		for i, percent := range utilization {
			id := fmt.Sprintf("CPUutilization%d", i)
			err_ = p.store.Put(ctx, schema.NewGauge(id, percent))
			if err_ != nil {
				return err_
			}
		}
		return nil
	}))

	err = eg.Wait()
	if err != nil {
		return nil, err
	}
	return p.store.List(ctx)
}

func (p Poller) Reset(ctx context.Context) error {
	return p.store.Put(ctx, schema.NewCounter(pollCount, p.start))
}
