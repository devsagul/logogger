package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v6"

	"logogger/internal/crypt"
	"logogger/internal/dumper"
	"logogger/internal/schema"
	"logogger/internal/server"
	"logogger/internal/storage"
	"logogger/internal/utils"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

type config struct {
	RawStoreInterval string        `json:"store_interval"`
	Address          string        `env:"ADDRESS" json:"address"`
	ConfigFilePath   string        `enc:"CONFIG"`
	CryptoKey        string        `env:"CRYPTO_KEY" json:"crypto_key"`
	StoreFile        string        `env:"STORE_FILE" json:"store_file"`
	Key              string        `env:"KEY" json:"key"`
	DatabaseDSN      string        `env:"DATABASE_DSN" json:"database_dsn"`
	TrustedSubnet    string        `env:"TRUSTED_SUBNET" json:"trusted_subnet"`
	StoreInterval    time.Duration `env:"STORE_INTERVAL"`
	Restore          bool          `env:"RESTORE" json:"restore"`
	Protocol         string        `env:"PROTOCOL" json:"protocol"`
}

var cfg config

func init() {
	flag.StringVar(&cfg.Address, "a", "localhost:8080", "Address of the server (to listen to)")
	flag.StringVar(&cfg.CryptoKey, "crypto-key", "", "Path to file with private encryption key")
	flag.DurationVar(&cfg.StoreInterval, "i", 300*time.Second, "Interval for storage state to be dumped on disk")
	flag.StringVar(&cfg.StoreFile, "f", "/tmp/devops-metrics-db.json", "Path to the file for dumping storage state")
	flag.BoolVar(&cfg.Restore, "r", true, "Restore store state from dump file on server initialization")
	flag.StringVar(&cfg.Key, "k", "", "Secret key to sign metrics (should be shared between server and agent)")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "Database connection string")
	flag.StringVar(&cfg.ConfigFilePath, "c", "", "Path to JSON configuration")
	flag.StringVar(&cfg.TrustedSubnet, "t", "", "CIDR representation of trusted subnet")
	flag.StringVar(&cfg.Protocol, "p", "http", "Communication protocol (http/grpc)")
}

func main() {
	utils.PrintVersionInfo(buildVersion, buildDate, buildCommit)
	log.Println("Initializing server...")
	flag.Parse()
	err := env.Parse(&cfg)
	if err != nil {
		log.Fatal("Could not parse config : ", err)
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
		cfg.StoreInterval, err = time.ParseDuration(cfg.RawStoreInterval)
		if err != nil {
			log.Fatal("Could not parse config file : ", err)
		}
	}

	// do it again to preserve order
	flag.Parse()
	err = env.Parse(&cfg)
	if err != nil {
		log.Fatal("Could not parse config : ", err)
	}
	if cfg.StoreInterval < 0 {
		log.Fatal("Invalid value for store interval")
	}
	log.Printf("DSN: %v", cfg.DatabaseDSN)

	decryptor, err := crypt.NewDecryptor(cfg.CryptoKey)
	if err != nil {
		log.Println("Could not setup encryption")
		os.Exit(1)
	}

	_, trustedSubnet, err := net.ParseCIDR(cfg.TrustedSubnet)
	if err != nil {
		log.Println("Could not setup encryption")
		os.Exit(1)
	}

	var store storage.MetricsStorage
	if cfg.DatabaseDSN != "" {
		log.Println("Initializing postgres database")
		store, err = storage.NewPostgresStorage(cfg.DatabaseDSN)
		if err != nil {
			log.Fatalf("error during storage initialization: %s", err.Error())
		}
	} else {
		store = storage.NewMemStorage()
	}
	defer func() {
		err = store.Close()
		if err != nil {
			log.Printf("Could not close the storage: %s", err.Error())
		}
	}()

	// restore storage if needed
	if cfg.Restore && cfg.DatabaseDSN == "" {
		log.Println("Restoring storage from file...")
		func() {
			f, err_ := os.OpenFile(cfg.StoreFile, os.O_RDONLY|os.O_CREATE, 0644)
			if err_ != nil {
				log.Fatal("Could not open file : ", err_)
			}

			buf := bytes.NewBuffer(nil)
			n, err_ := io.Copy(buf, f)
			if n == 0 {
				// file is empty, valid scenario
				// nothing to restore
				return
			}
			if err_ != nil {
				log.Fatal("Could not read data : ", err_)
			}

			var l []schema.Metrics
			err_ = json.Unmarshal(buf.Bytes(), &l)
			if err_ != nil {
				log.Fatal("Could not restore data : ", err_)
			}

			err_ = store.BulkPut(context.Background(), l)
			if err_ != nil {
				log.Fatal("Could not save restored data : ", err_)
			}

			err_ = f.Close()
			if err_ != nil {
				log.Fatal("Could not close dump file : ", err_)
			}
		}()
	}

	log.Println("Initializing dumper...")
	d := dumper.NewSyncDumper(cfg.StoreFile)

	defer func() {
		err_ := d.Close()
		if err_ != nil {
			log.Fatal("Error Closing dumper : ", err_)
		}
	}()

	log.Println("Initializing application...")
	app := server.NewApp(store).WithDumper(d).WithDumpInterval(cfg.StoreInterval).WithKey(cfg.Key)

	var srv server.Server

	if cfg.Protocol == "grpc" {
		srv = server.NewGrpcServer(app)
	} else {
		srv = server.NewHttpServer(app)
	}
	srv = srv.WithDecryptor(decryptor).WithTrustedSubnet(trustedSubnet)
	log.Println("Listening...")

	idleConnsClosed := make(chan struct{})
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigint
		err = srv.Shutdown(idleConnsClosed)
		if err != nil {
			log.Fatal("Error Shutting down the Server : ", err)
		}
	}()

	err = srv.Serve(cfg.Address)
	if err != nil {
		log.Fatal("Error Starting the Server : ", err)
	}
	<-idleConnsClosed
	fmt.Println("Server Shutdown gracefully")
}
