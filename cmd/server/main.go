package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/caarlos0/env/v6"

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
	Address       string        `env:"ADDRESS"`
	StoreFile     string        `env:"STORE_FILE"`
	Key           string        `env:"KEY"`
	DatabaseDSN   string        `env:"DATABASE_DSN"`
	StoreInterval time.Duration `env:"STORE_INTERVAL"`
	Restore       bool          `env:"RESTORE"`
}

var cfg config

func init() {
	flag.StringVar(&cfg.Address, "a", "localhost:8080", "Address of the server (to listen to)")
	flag.DurationVar(&cfg.StoreInterval, "i", 300*time.Second, "Interval for storage state to be dumped on disk")
	flag.StringVar(&cfg.StoreFile, "f", "/tmp/devops-metrics-db.json", "Path to the file for dumping storage state")
	flag.BoolVar(&cfg.Restore, "r", true, "Restore store state from dump file on server initialization")
	flag.StringVar(&cfg.Key, "k", "", "Secret key to sign metrics (should be shared between server and agent)")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "Database connection string")
}

func main() {
	utils.PrintVersionInfo(buildVersion, buildDate, buildCommit)
	log.Println("Initializing server...")
	log.Printf("%v", os.Args)
	flag.Parse()
	err := env.Parse(&cfg)
	if err != nil {
		log.Fatal("Could not parse config : ", err)
	}
	if cfg.StoreInterval < 0 {
		log.Fatal("Invalid value for store interval")
	}
	log.Printf("DSN: %v", cfg.DatabaseDSN)

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
	log.Println("Listening...")
	err = http.ListenAndServe(cfg.Address, app.Router)
	if err != nil {
		log.Fatal("Error Starting the HTTP Server : ", err)
	}
}
