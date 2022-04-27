package main

import (
	"log"
	"logogger/internal/server"
	"net/http"
)

func main() {
	app := server.NewApp()
	err := http.ListenAndServe("127.0.0.1:8080", app.Router)
	if err != nil {
		log.Fatal("Error Starting the HTTP Server : ", err)
		return
	}
}
