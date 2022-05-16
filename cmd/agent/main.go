package main

import (
	"fmt"
	"log"
	"logogger/internal/poller"
	"logogger/internal/reporter"
	"logogger/internal/schema"
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
	time.Sleep(time.Second)

	pollTicker := time.NewTicker(pollInterval)
	reportTicker := time.NewTicker(reportInterval)
	channel := make(chan []schema.Metrics)
	p, err := poller.NewPoller(0)
	if err != nil {
		log.Println("Could not initialize poller")
		os.Exit(1)
	}

	go func() {
		for {
			<-pollTicker.C
			l, err := p.Poll()
			if err != nil {
				log.Println("Unable to poll data")
				time.Sleep(time.Second)
			} else {
				channel <- l
			}
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var l []schema.Metrics
	func() {
		var run = true
		for run {
			select {
			case metrics := <-channel:
				l = metrics
			case <-reportTicker.C:
				err := reporter.ReportMetrics(l, reportHost)
				os.Exit(0)
				if err == nil {
					err = p.Reset()
					if err != nil {
						fmt.Println("Unable to reset PollCount")
					}
				} else {
					fmt.Println("Unable to send metrics to server: %s", err.Error())
				}
			case <-sigs:
				fmt.Println("Exiting agent gracefully...")
				run = false
			}
		}
	}()
}
