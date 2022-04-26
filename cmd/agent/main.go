package main

import (
	"fmt"
	"logogger/internal/poller"
	"logogger/internal/reporter"
	"time"
)

const (
	pollInterval   = 2 * time.Second
	reportInterval = 10 * time.Second
	reportHost     = "127.0.0.1:8080"
)

func main() {
	pollTicker := time.NewTicker(pollInterval)
	reportTicker := time.NewTicker(reportInterval)
	channel := make(chan poller.Metrics)
	responsesChannel := make(chan reporter.ServerResponse)

	go func() {
		p := poller.Poller(0)
		for {
			<-pollTicker.C
			channel <- p()
		}
	}()

	var m poller.Metrics

	func() {
		for {
			select {
			case metrics := <-channel:
				m = metrics
			case <-reportTicker.C:
				reporter.ReportMetrics(m, reportHost, responsesChannel)
			case r := <-responsesChannel:
				fmt.Println(r)
			}
		}
	}()
}
