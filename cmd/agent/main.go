package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"
	"golang.org/x/sync/errgroup"

	"logogger/internal/poller"
	"logogger/internal/reporter"
	"logogger/internal/schema"
	"logogger/internal/utils"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

type config struct {
	ReportHost     string        `env:"ADDRESS"`
	Key            string        `env:"KEY"`
	PollInterval   time.Duration `env:"POLL_INTERVAL"`
	ReportInterval time.Duration `env:"REPORT_INTERVAL"`
}

var cfg config

func init() {
	flag.DurationVar(&cfg.PollInterval, "p", 2*time.Second, "Interval of metrics polling")
	flag.DurationVar(&cfg.ReportInterval, "r", 10*time.Second, "Interval of metrics reporting")
	flag.StringVar(&cfg.ReportHost, "a", "localhost:8080", "Address of the server to report metrics to")
	flag.StringVar(&cfg.Key, "k", "", "Secret key to sign metrics (should be shared between server and agent)")
}

func main() {
	utils.PrintVersionInfo(buildVersion, buildDate, buildCommit)
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
	ctx := context.Background()
	p, err := poller.NewPoller(ctx, 0)
	if err != nil {
		log.Println("Could not initialize poller")
		os.Exit(1)
	}

	var metrics []schema.Metrics

	go utils.RetryForever(utils.WrapGoroutinePanic(func() error {
		for {
			<-pollTicker.C
			l, err := p.Poll(ctx)
			if err != nil {
				log.Println("Unable to poll data")
				time.Sleep(time.Second)
			} else {
				// we store metrics only if poll was reliable
				metrics = l
			}
		}
	}), time.Minute)()

	go utils.RetryForever(utils.WrapGoroutinePanic(func() error {
		for {
			<-reportTicker.C
			err := report(metrics, reportHost, cfg.Key)
			if err == nil {
				err = p.Reset(ctx)
				if err != nil {
					log.Println("Unable to reset PollCount")
				}
			} else {
				log.Printf("Unable to send metrics to server: %s\n", err.Error())
			}
		}
	}), time.Minute)()

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
			eg.Go(utils.WrapGoroutinePanic(func() error {
				err := m.Sign(key)
				if err != nil {
					return err
				}
				signed = append(signed, m)
				return nil
			}))
		}
		if err := eg.Wait(); err != nil {
			return err
		}
		l = signed
	}

	err := reporter.ReportMetricsBatches(l, host)
	return err
}
