package poller

import (
	"math/rand"
	"runtime"
	"sync"
)

type gauge float64
type counter int64

type Gauge = gauge
type Counter = counter

type Metrics struct {
	Alloc         gauge
	BuckHashSys   gauge
	Frees         gauge
	GCCPUFraction gauge
	GCSys         gauge
	HeapAlloc     gauge
	HeapIdle      gauge
	HeapInuse     gauge
	HeapObjects   gauge
	HeapReleased  gauge
	HeapSys       gauge
	LastGC        gauge
	Lookups       gauge
	MCacheInuse   gauge
	MCacheSys     gauge
	MSpanInuse    gauge
	MSpanSys      gauge
	Mallocs       gauge
	NextGC        gauge
	NumForcedGC   gauge
	NumGC         gauge
	OtherSys      gauge
	PauseTotalNs  gauge
	StackInuse    gauge
	StackSys      gauge
	Sys           gauge
	TotalAlloc    gauge
	PollCount     counter
	RandomValue   gauge
}

func sequence(start int64) (func() int64, chan struct{}) {
	i := start
	reset := make(chan struct{})
	mu := sync.Mutex{}

	go func() {
		for {
			<-reset
			mu.Lock()
			i = start
			mu.Unlock()
		}
	}()

	return func() int64 {
		mu.Lock()
		defer mu.Unlock()
		i++
		return i
	}, reset
}

func Poller(start int64) (func() Metrics, chan struct{}) {
	c, reset := sequence(start)

	return func() Metrics {
		var memStats runtime.MemStats

		runtime.ReadMemStats(&memStats)

		return Metrics{
			Alloc:         gauge(memStats.Alloc),
			BuckHashSys:   gauge(memStats.BuckHashSys),
			Frees:         gauge(memStats.Frees),
			GCCPUFraction: gauge(memStats.GCCPUFraction),
			GCSys:         gauge(memStats.GCSys),
			HeapAlloc:     gauge(memStats.HeapAlloc),
			HeapIdle:      gauge(memStats.HeapIdle),
			HeapInuse:     gauge(memStats.HeapInuse),
			HeapObjects:   gauge(memStats.HeapObjects),
			HeapReleased:  gauge(memStats.HeapReleased),
			HeapSys:       gauge(memStats.HeapSys),
			LastGC:        gauge(memStats.LastGC),
			Lookups:       gauge(memStats.Lookups),
			MCacheInuse:   gauge(memStats.MCacheInuse),
			MCacheSys:     gauge(memStats.MCacheSys),
			MSpanInuse:    gauge(memStats.MSpanInuse),
			Mallocs:       gauge(memStats.Mallocs),
			NextGC:        gauge(memStats.NextGC),
			NumForcedGC:   gauge(memStats.NumForcedGC),
			NumGC:         gauge(memStats.NumGC),
			OtherSys:      gauge(memStats.OtherSys),
			PauseTotalNs:  gauge(memStats.PauseTotalNs),
			StackInuse:    gauge(memStats.StackInuse),
			StackSys:      gauge(memStats.StackSys),
			Sys:           gauge(memStats.Sys),
			TotalAlloc:    gauge(memStats.TotalAlloc),
			PollCount:     counter(c()),
			RandomValue:   gauge(rand.Float64()),
		}
	}, reset
}
