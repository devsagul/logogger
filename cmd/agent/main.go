package main

import (
	"fmt"
	"logogger/internal/poller"
	"logogger/internal/reporter"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	pollInterval   = 2 * time.Second
	reportInterval = 10 * time.Second
	reportHost     = "http://127.0.0.1:8080"
)

func main() {
	pollTicker := time.NewTicker(pollInterval)
	reportTicker := time.NewTicker(reportInterval)
	channel := make(chan poller.Metrics)
	p, reset := poller.Poller(0)

	go func() {
		for {
			<-pollTicker.C
			channel <- p()
		}
	}()

	var m poller.Metrics
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	func() {
		var run = true
		for run {
			select {
			case metrics := <-channel:
				m = metrics
			case <-reportTicker.C:
				err := reporter.ReportMetrics(m, reportHost)
				if err == nil {
					reset()
				}
			case <-sigs:
				fmt.Println("Exiting agent...")
				run = false
			}
		}
	}()
}
