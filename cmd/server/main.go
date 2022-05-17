package main

import (
	"github.com/caarlos0/env/v6"
	"log"
	"logogger/internal/server"
	"logogger/internal/storage"
	"net/http"
	"os"
)

type config struct {
	Address string `env:"ADDRESS" envDefault:"localhost:8080"`
}

func main() {
	var cfg config
	err := env.Parse(&cfg)
	if err != nil {
		log.Println("Could not parse config")
		os.Exit(1)
	}

	app := server.NewApp(storage.NewMemStorage())
	err = http.ListenAndServe(cfg.Address, app.Router)
	if err != nil {
		log.Fatal("Error Starting the HTTP Server : ", err)
		return
	}
}
