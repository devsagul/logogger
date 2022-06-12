package main

import (
	"flag"
	"fmt"
	"log"
	"logogger/internal/poller"
	"logogger/internal/reporter"
	"logogger/internal/schema"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

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
	p, err := poller.NewPoller(0)
	if err != nil {
		log.Println("Could not initialize poller")
		os.Exit(1)
	}

	var metrics []schema.Metrics

	go func() {
		for {
			<-pollTicker.C
			l, err := p.Poll()
			if err != nil {
				log.Println("Unable to poll data")
				time.Sleep(time.Second)
			} else {
				// we store metrics only if poll was reliable
				metrics = l
			}
		}
	}()

	go func() {
		for {
			<-reportTicker.C
			err := report(metrics, reportHost, cfg.Key)
			if err == nil {
				err = p.Reset()
				if err != nil {
					log.Println("Unable to reset PollCount")
				}
			} else {
				log.Printf("Unable to send metrics to server: %s\n", err.Error())
			}
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-sigs
	log.Println("Exiting agent gracefully...")
}

func report(l []schema.Metrics, host string, key string) error {
	if key != "" {
		eg := errgroup.Group{}
		var signed []schema.Metrics
		for _, m := range l {
			m := m
			eg.Go(func() error {
				err := m.Sign(key)
				if err != nil {
					return err
				}
				signed = append(signed, m)
				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			return err
		}
		l = signed
	}

	err := reporter.ReportMetricsBatches(l, host)
	return err
}
