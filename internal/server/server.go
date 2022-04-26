package server

import (
	"fmt"
	"log"
	"logogger/internal/router"
	"logogger/internal/storage"
	"net/http"
	"strconv"
)

type App struct {
	store storage.MetricsStorage
	r     *router.Router
}

func (app App) handleGauge(w http.ResponseWriter, r *http.Request, args []string) {
	name := args[0]
	rawValue := args[1]
	var body string
	defer log.Printf("URL: %s, response: %s", r.URL.Path, body)

	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		body = fmt.Sprintf("Status: ERROR\nCouldn't parse float from %s", rawValue)
		_, _ = w.Write([]byte(body))
		return
	}

	err = app.store.SetGauge(name, value)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		body = "Internal Server Error"
		_, _ = w.Write([]byte(body))
		return
	}

	w.WriteHeader(http.StatusOK)
	body = "Status: OK"
	_, _ = w.Write([]byte(body))
}

func (app App) handleCounter(w http.ResponseWriter, r *http.Request, args []string) {
	name := args[0]
	rawValue := args[1]
	var body string
	defer log.Printf("URL: %s, response: %s", r.URL.Path, body)

	value, err := strconv.ParseInt(rawValue, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		body = fmt.Sprintf("Status: ERROR\nCouldn't parse float from %s", rawValue)
		_, _ = w.Write([]byte(body))
		return
	}

	err = app.store.IncrementCounter(name, value)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		body = "Internal Server Error"
		_, _ = w.Write([]byte(body))
		return
	}

	w.WriteHeader(http.StatusOK)
	body = "Status: OK"
	_, _ = w.Write([]byte(body))
}

func NewApp() *App {
	app := new(App)
	app.r = new(router.Router)
	app.store = storage.NewMemStorage()
	app.r.RegisterHandler(`/update/gauge/(?P<Name>\w+)/(?P<Value>[^/]+)$`, app.handleGauge)
	app.r.RegisterHandler(`/update/counter/(?P<Name>\w+)/(?P<Value>[^/]+)$`, app.handleCounter)
	return app
}

func (app App) Handle(w http.ResponseWriter, r *http.Request) {
	app.r.Handle(w, r)
}
