package server

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"logogger/internal/schema"
	"logogger/internal/storage"
	"net/http"
	"strings"
	"time"
)

type App struct {
	store  storage.MetricsStorage
	Router *chi.Mux
}

func (app App) getValue(w http.ResponseWriter, r *http.Request) {
	valueType := chi.URLParam(r, "Type")
	name := chi.URLParam(r, "Name")

	ctx := r.Context()
	errChan := make(chan error)
	read := make(chan schema.Metrics)

	go func() {
		var req schema.Metrics
		switch valueType {
		case "counter":
			req = schema.NewCounterRequest(name)
		case "gauge":
			req = schema.NewCounterRequest(name)
		default:
			errChan <- &requestError{
				status: http.StatusNotImplemented,
				body:   fmt.Sprintf("Status: ERROR\nCould not perform requested operation on metric type %s", valueType),
			}
			return
		}

		value, err := app.store.Extract(req)
		if err != nil {
			errChan <- err
			return
		}
		read <- value
	}()

	select {
	case value := <-read:
		_, _, body := value.Explain()
		safeWrite(w, http.StatusOK, body)
		return
	case err := <-errChan:
		writeError(w, err)
	case <-ctx.Done():
		return
	}
}

func (app App) updateValue(w http.ResponseWriter, r *http.Request) {
	valueType := chi.URLParam(r, "Type")
	name := chi.URLParam(r, "Name")
	rawValue := chi.URLParam(r, "Value")

	ctx := r.Context()
	errChan := make(chan error)
	done := make(chan struct{})

	go func() {
		value, err := parseMetric(valueType, name, rawValue)
		if err != nil {
			errChan <- err
			return
		}
		if value.MType == "counter" {
			err = app.store.Increment(value, *value.Delta)
			_, ok := err.(*storage.NotFound)
			if ok {
				err = app.store.Put(value)
			}
		} else {
			err = app.store.Put(value)
		}
		if err != nil {
			errChan <- err
			return
		}
		done <- struct{}{}
	}()

	select {
	case <-done:
		safeWrite(w, http.StatusOK, "Status: OK")
		return
	case err := <-errChan:
		writeError(w, err)
		return
	case <-ctx.Done():
		return
	}
}

func (app App) listMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	read := make(chan []schema.Metrics)
	errChan := make(chan error)

	go func() {
		list, err := app.store.List()
		if err != nil {
			errChan <- err
			return
		}
		read <- list
	}()

	select {
	case list := <-read:
		w.Header().Set("Content-Type", "text/html")
		var sb strings.Builder

		header := "<table><tr><th>Type</th><th>Name</th><th>Value</th></tr>"
		sb.Write([]byte(header))
		for _, metrics := range list {
			name, mType, value := metrics.Explain()
			row := fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td></tr>", name, mType, value)
			sb.Write([]byte(row))
		}
		footer := "</table>"
		sb.Write([]byte(footer))
		safeWrite(w, http.StatusOK, sb.String())
		return
	case err := <-errChan:
		writeError(w, err)
		return
	case <-ctx.Done():
		return
	}
}

func NewApp() *App {
	app := new(App)
	r := chi.NewRouter()
	app.Router = r
	app.store = storage.NewMemStorage()

	// useful middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.SetHeader("Content-Type", "text/plain"))

	r.Post("/update/{Type}/{Name}/{Value}", app.updateValue)
	r.Get("/value/{Type}/{Name}", app.getValue)
	r.Get("/", app.listMetrics)
	return app
}
