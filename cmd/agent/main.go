package main

import (
	"context"
	"encoding/json"
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

	"logogger/internal/crypt"
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
	RawPollInterval   string        `json:"poll_interval"`
	RawReportInterval string        `json:"report_interval"`
	ConfigFilePath    string        `enc:"CONFIG"`
	CryptoKey         string        `env:"CRYPTO_KEY" json:"crypto_key"`
	ReportHost        string        `env:"ADDRESS" json:"report_host"`
	Key               string        `env:"KEY" json:"key"`
	PollInterval      time.Duration `env:"POLL_INTERVAL"`
	ReportInterval    time.Duration `env:"REPORT_INTERVAL"`
}

var cfg config

func init() {
	flag.DurationVar(&cfg.PollInterval, "p", 2*time.Second, "Interval of metrics polling")
	flag.DurationVar(&cfg.ReportInterval, "r", 10*time.Second, "Interval of metrics reporting")
	flag.StringVar(&cfg.CryptoKey, "crypto-key", "", "Path to file with public encryption key")
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

	if len(cfg.ConfigFilePath) > 0 {
		data, err := os.ReadFile(cfg.ConfigFilePath)
		if err != nil {
			log.Fatal("Could not read config file : ", err)
		}
		err = json.Unmarshal(data, &cfg)
		if err != nil {
			log.Fatal("Could not parse config file : ", err)
		}
		cfg.PollInterval, err = time.ParseDuration(cfg.RawPollInterval)
		if err != nil {
			log.Fatal("Could not parse config file : ", err)
		}
		cfg.ReportInterval, err = time.ParseDuration(cfg.RawReportInterval)
		if err != nil {
			log.Fatal("Could not parse config file : ", err)
		}
	}

	// preserve order
	flag.Parse()
	err = env.Parse(&cfg)
	if err != nil {
		log.Println("Could not parse config")
		os.Exit(1)
	}

	encryptor, err := crypt.NewEncryptor(cfg.CryptoKey)
	if err != nil {
		log.Println("Could not setup encryption")
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

	reporter := reporter.NewReporter(encryptor)

	go utils.RetryForever(utils.WrapGoroutinePanic(func() error {
		for {
			<-reportTicker.C
			err := report(reporter, metrics, reportHost, cfg.Key)
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
	pollTicker.Stop()
	reportTicker.Stop()
	reporter.Shutdown()
}

func report(poller *reporter.Reporter, l []schema.Metrics, host string, key string) error {
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

	err := poller.ReportMetricsBatches(l, host)
	return err
}
