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

func (app App) handleRequest(w http.ResponseWriter, r *http.Request, args []string) {
	valueType := args[0]
	name := args[1]
	rawValue := args[2]
	var body string
	defer log.Printf("URL: %s, response: %s", r.URL.Path, body)

	if valueType == "counter" {
		if r.Method == http.MethodGet {
			value, found, err := app.store.GetCounter(name)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				body = "Internal Server Error"
				_, _ = w.Write([]byte(body))
				return
			} else if !found {
				w.WriteHeader(http.StatusNotFound)
				body = "NotFound"
				_, _ = w.Write([]byte(body))
				return
			}
			w.WriteHeader(http.StatusOK)
			body = fmt.Sprintf("%d", value)
			_, _ = w.Write([]byte(body))
			return
		}
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
	} else if valueType == "gauge" {
		if r.Method == http.MethodGet {
			value, found, err := app.store.GetGauge(name)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				body = "Internal Server Error"
				_, _ = w.Write([]byte(body))
				return
			} else if !found {
				w.WriteHeader(http.StatusNotFound)
				body = "NotFound"
				_, _ = w.Write([]byte(body))
				return
			}
			w.WriteHeader(http.StatusOK)
			body = fmt.Sprintf("%f", value)
			_, _ = w.Write([]byte(body))
			return
		}
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
	} else {
		w.WriteHeader(http.StatusNotImplemented)
		body = fmt.Sprintf("Status: ERROR\nUnknown metric type %s", name)
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
	app.r.RegisterHandler(`/update/(?P<Type\w+>)/(?P<Name>\w+)/(?P<Value>[^/]+)$`, app.handleRequest)
	return app
}

func (app App) Handle(w http.ResponseWriter, r *http.Request) {
	app.r.Handle(w, r)
}
