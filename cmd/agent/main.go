package main

import (
	"flag"
	"fmt"
	"golang.org/x/sync/errgroup"
	"log"
	"logogger/internal/poller"
	"logogger/internal/reporter"
	"logogger/internal/schema"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
)

type config struct {
	PollInterval   time.Duration `env:"POLL_INTERVAL"`
	ReportInterval time.Duration `env:"REPORT_INTERVAL"`
	ReportHost     string        `env:"ADDRESS"`
	Key            string        `env:"KEY"`
}

var cfg config

func init() {
	flag.DurationVar(&cfg.PollInterval, "p", 2*time.Second, "Interval of metrics polling")
	flag.DurationVar(&cfg.ReportInterval, "r", 10*time.Second, "Interval of metrics reporting")
	flag.StringVar(&cfg.ReportHost, "a", "localhost:8080", "Address of the server to report metrics to")
	flag.StringVar(&cfg.Key, "k", "", "Secret key to sign metrics (should be shared between server and agent)")
}

func main() {
	flag.Parse()
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
				err := report(l, reportHost, cfg.Key)
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

func report(l []schema.Metrics, host string, key string) error {
	if key != "" {
		eg := errgroup.Group{}
		for _, m := range l {
			m := &m
			eg.Go(func() error {
				return m.Sign(key)
			})
		}
		if err := eg.Wait(); err != nil {
			return err
		}
	}

	err := reporter.ReportMetrics(l, host)
	return err
}
