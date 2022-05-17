package main

import (
	"bytes"
	"encoding/json"
	"github.com/caarlos0/env/v6"
	"io"
	"log"
	"logogger/internal/dumper"
	"logogger/internal/schema"
	"logogger/internal/server"
	"logogger/internal/storage"
	"net/http"
	"os"
	"time"
)

type config struct {
	Address       string `env:"ADDRESS" envDefault:"localhost:8080"`
	StoreInterval int64  `env:"STORE_INTERVAL" envDefault:"300"`
	StoreFile     string `env:"STORE_FILE" envDefault:"/tmp/devops-metrics-db.json"`
	Restore       bool   `env:"RESTORE" envDefault:"true"`
}

func main() {
	log.Println("Initializing server...")
	var cfg config
	err := env.Parse(&cfg)
	if err != nil {
		log.Fatal("Could not parse config : ", err)
	}
	if cfg.StoreInterval < 0 {
		log.Fatal("Invalid value for store interval")
	}

	store := storage.NewMemStorage()

	// restore storage if needed
	if cfg.Restore {
		log.Println("Restoring storage from file...")
		func() {
			f, err := os.OpenFile(cfg.StoreFile, os.O_RDONLY|os.O_CREATE, 0644)
			if err != nil {
				log.Fatal("Could not open file : ", err)
			}

			buf := bytes.NewBuffer(nil)
			n, err := io.Copy(buf, f)
			if n == 0 {
				// file is empty, valid scenario
				// nothing to restore
				return
			}
			if err != nil {
				log.Fatal("Could not read data : ", err)
			}

			var l []schema.Metrics
			err = json.Unmarshal(buf.Bytes(), &l)
			if err != nil {
				log.Fatal("Could not restore data : ", err)
			}

			err = store.BulkPut(l)
			if err != nil {
				log.Fatal("Could not save restored data : ", err)
			}

			err = f.Close()
			if err != nil {
				log.Fatal("Could not close dump file : ", err)
			}
		}()
	}

	log.Println("Initializing dumper...")
	d := dumper.NewSyncDumper(cfg.StoreFile)

	defer func() {
		err := d.Close()
		if err != nil {
			log.Fatal("Error Closing dumper : ", err)
		}
	}()

	duration := time.Duration(cfg.StoreInterval) * time.Second
	log.Println("Initializing application...")
	app := server.NewApp(store).WithDumper(d).WithDumpInterval(duration)
	log.Println("Listening...")
	err = http.ListenAndServe(cfg.Address, app.Router)
	if err != nil {
		log.Fatal("Error Starting the HTTP Server : ", err)
	}
}
