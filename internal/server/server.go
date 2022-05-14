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

func (app App) RetrieveValue(w http.ResponseWriter, r *http.Request) {
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
			req = schema.NewGaugeRequest(name)
		default:
			errChan <- &requestError{
				status: http.StatusNotImplemented,
				body:   fmt.Sprintf("Could not perform requested operation on metric type %s", valueType),
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
		SafeWrite(w, http.StatusOK, body)
		return
	case err := <-errChan:
		WriteError(w, err)
	case <-ctx.Done():
		return
	}
}

func (app App) UpdateValue(w http.ResponseWriter, r *http.Request) {
	valueType := chi.URLParam(r, "Type")
	name := chi.URLParam(r, "Name")
	rawValue := chi.URLParam(r, "Value")

	ctx := r.Context()
	errChan := make(chan error)
	done := make(chan struct{})

	go func() {
		value, err := ParseMetric(valueType, name, rawValue)
		if err != nil {
			errChan <- err
			return
		}
		switch value.MType {
		case "counter":
			err = app.store.Increment(value, *value.Delta)
			switch err.(type) {
			case *storage.NotFound:
				err = app.store.Put(value)
			}
		case "gauge":
			err = app.store.Put(value)
		default:
			errChan <- &requestError{
				status: http.StatusNotImplemented,
				body:   fmt.Sprintf("Could not perform requested operation on metric type %s", valueType),
			}
			return
		}
		if err != nil {
			errChan <- err
			return
		}
		done <- struct{}{}
	}()

	select {
	case <-done:
		SafeWrite(w, http.StatusOK, "Status: OK")
		return
	case err := <-errChan:
		WriteError(w, err)
		return
	case <-ctx.Done():
		return
	}
}

func (app App) ListMetrics(w http.ResponseWriter, r *http.Request) {
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
		if len(list) == 0 {
			w.WriteHeader(http.StatusOK)
			return
		}
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
		SafeWrite(w, http.StatusOK, sb.String())
		return
	case err := <-errChan:
		WriteError(w, err)
		return
	case <-ctx.Done():
		return
	}
}

func NewApp(store storage.MetricsStorage) *App {
	app := new(App)
	r := chi.NewRouter()
	app.Router = r
	app.store = store

	// useful middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.SetHeader("Content-Type", "text/plain"))

	r.Post("/update/{Type}/{Name}/{Value}", app.UpdateValue)
	r.Get("/value/{Type}/{Name}", app.RetrieveValue)
	r.Get("/", app.ListMetrics)
	return app
}
