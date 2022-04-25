package poller

import (
       "math/rand"
       "runtime"
)

type gauge float64
type counter int64

type Metrics struct {
     Alloc gauge
     BuckHashSys gauge
     Frees gauge
     GCCPUFraction gauge
     GCSys gauge
     HeapAlloc gauge
     HeapIdle gauge
     HeapInuse gauge
     HeapObjects gauge
     HeapReleased gauge
     HeapSys gauge
     LastGC gauge
     Lookups gauge
     MCacheInuse gauge
     MCacheSys gauge
     MSpanInuse gauge
     Mallocs gauge
     NextGC gauge
     NumForcedGC gauge
     NumGC gauge
     OtherSys gauge
     PauseTotalNs gauge
     StackInuse gauge
     StackSys gauge
     Sys gauge
     TotalAlloc gauge
     PollCount counter
     RandomValue gauge
}

func Sequence(start int64) func() int64 {
      i := start
      return func () int64 {
      	     i++
	     return i
      }
}

func Poller(start int64) func() Metrics {
     c := Sequence(start)
     
     return func() Metrics {
     	    var memstats runtime.MemStats

	    runtime.ReadMemStats(&memstats)

	    return Metrics{
	    	   Alloc: gauge(memstats.Alloc),
		   BuckHashSys: gauge(memstats.BuckHashSys),
		   Frees: gauge(memstats.Frees),
		   GCCPUFraction: gauge(memstats.GCCPUFraction),
		   GCSys: gauge(memstats.GCSys),
		   HeapAlloc: gauge(memstats.HeapAlloc),
		   HeapIdle: gauge(memstats.HeapIdle),
		   HeapInuse: gauge(memstats.HeapInuse),
		   HeapObjects: gauge(memstats.HeapObjects),
		   HeapReleased: gauge(memstats.HeapReleased),
		   HeapSys: gauge(memstats.HeapSys),
		   LastGC: gauge(memstats.LastGC),
		   Lookups: gauge(memstats.Lookups),
		   MCacheInuse: gauge(memstats.MCacheInuse),
     		   MCacheSys: gauge(memstats.MCacheSys),
     		   MSpanInuse: gauge(memstats.MSpanInuse),
     		   Mallocs: gauge(memstats.Mallocs),
     		   NextGC: gauge(memstats.NextGC),
     		   NumForcedGC: gauge(memstats.NumForcedGC),
     		   NumGC: gauge(memstats.NumGC),
     		   OtherSys: gauge(memstats.OtherSys),
     		   PauseTotalNs: gauge(memstats.PauseTotalNs),
     		   StackInuse: gauge(memstats.StackInuse),
     		   StackSys: gauge(memstats.StackSys),
     		   Sys: gauge(memstats.Sys),
     		   TotalAlloc: gauge(memstats.TotalAlloc),
		   PollCount: counter(c()),
		   RandomValue: gauge(rand.Float64()),
	    }
     }
}
