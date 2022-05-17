package main

import (
	"fmt"
	"github.com/caarlos0/env/v6"
	"log"
	"logogger/internal/poller"
	"logogger/internal/reporter"
	"logogger/internal/schema"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"
)

type config struct {
	PollInterval   time.Duration `env:"POLL_INTERVAL" envDefault:"2s"`
	ReportInterval time.Duration `env:"REPORT_INTERVAL" envDefault:"10s"`
	ReportHost     string        `env:"REPORT_HOST" envDefault:"localhost:8080"`
}

func main() {
	var cfg config
	err := env.Parse(&cfg)
	if err != nil {
		log.Println("Could not parse config")
		os.Exit(1)
	}

	var reportHost = cfg.ReportHost

	r := regexp.MustCompile(`https?://`)
	if !r.MatchString(cfg.ReportHost) {
		reportHost = fmt.Sprintf("http://%s", reportHost)
	}

	pollTicker := time.NewTicker(cfg.PollInterval)
	reportTicker := time.NewTicker(cfg.ReportInterval)
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
				if err == nil {
					err = p.Reset()
					if err != nil {
						log.Println("Unable to reset PollCount")
					}
				} else {
					log.Printf("Unable to send metrics to server: %s\n", err.Error())
				}
			case <-sigs:
				log.Println("Exiting agent gracefully...")
				run = false
			}
		}
	}()
}
