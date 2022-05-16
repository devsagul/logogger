package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"io"
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

type errorHttpHandler func(http.ResponseWriter, *http.Request) error

func newHandler(handler errorHttpHandler) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		errChan := make(chan error)
		ctx := request.Context()

		go func() {
			errChan <- handler(writer, request)
		}()

		select {
		case err := <-errChan:
			if err != nil {
				WriteError(writer, err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (app App) retrieveValue(w http.ResponseWriter, r *http.Request) error {
	valueType := chi.URLParam(r, "Type")
	name := chi.URLParam(r, "Name")

	var req schema.Metrics
	switch valueType {
	case "counter":
		req = schema.NewCounterRequest(name)
	case "gauge":
		req = schema.NewGaugeRequest(name)
	default:
		return &requestError{
			status: http.StatusNotImplemented,
			body:   fmt.Sprintf("Could not perform requested operation on metric type %s", valueType),
		}
	}

	value, err := app.store.Extract(req)
	if err != nil {
		return err
	}

	_, _, body := value.Explain()
	SafeWrite(w, http.StatusOK, body)
	return nil
}

func (app App) updateValue(w http.ResponseWriter, r *http.Request) error {
	valueType := chi.URLParam(r, "Type")
	name := chi.URLParam(r, "Name")
	rawValue := chi.URLParam(r, "Value")

	value, err := ParseMetric(valueType, name, rawValue)
	if err != nil {
		return err
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
		return &requestError{
			status: http.StatusNotImplemented,
			body:   fmt.Sprintf("Could not perform requested operation on metric type %s", valueType),
		}
	}

	if err != nil {
		return err
	}

	SafeWrite(w, http.StatusOK, "Status: OK")
	return nil
}

func (app App) listMetrics(w http.ResponseWriter, r *http.Request) error {
	list, err := app.store.List()
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html")
	if len(list) == 0 {
		w.WriteHeader(http.StatusOK)
		return nil
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
	return nil
}

func (app App) updateValueJSON(w http.ResponseWriter, r *http.Request) error {
	if r.Body == nil {
		return ValidationError("empty body")
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var m schema.Metrics
	err = decoder.Decode(&m)
	if err != nil {
		return ValidationError(err.Error())
	}

	if m.MType == "counter" && m.Delta == nil || m.MType == "gauge" && m.Value == nil {
		return ValidationError("Missing Value")
	}

	switch m.MType {
	case "counter":
		err = app.store.Increment(m, *m.Delta)
		switch err.(type) {
		case *storage.NotFound:
			err = app.store.Put(m)
		}
	case "gauge":
		err = app.store.Put(m)
	default:
		return &requestError{
			status: http.StatusNotImplemented,
			body:   fmt.Sprintf("Could not perform requested operation on metric type %s", m.MType),
		}
	}

	if err != nil {
		return err
	}

	value, err := app.store.Extract(m)
	if err != nil {
		return err
	}

	serialized, err := json.Marshal(value)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	SafeWrite(w, http.StatusOK, string(serialized))
	return nil
}

func (app App) retrieveValueJSON(w http.ResponseWriter, r *http.Request) error {
	if r.Body == nil {
		return ValidationError("empty body")
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return ValidationError(err.Error())
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var m schema.Metrics
	err = decoder.Decode(&m)
	if err != nil {
		return ValidationError(err.Error())
	}

	var value schema.Metrics
	switch m.MType {
	case "counter":
		value, err = app.store.Extract(m)
	case "gauge":
		value, err = app.store.Extract(m)
	default:
		return &requestError{
			status: http.StatusNotImplemented,
			body:   fmt.Sprintf("Could not perform requested operation on metric type %s", m.MType),
		}
	}
	if err != nil {
		return err
	}

	serialized, err := json.Marshal(value)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	SafeWrite(w, http.StatusOK, string(serialized))
	return nil
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

	r.Post("/update/{Type}/{Name}/{Value}", newHandler(app.updateValue))
	r.Get("/value/{Type}/{Name}", newHandler(app.retrieveValue))
	r.Post("/update/", newHandler(app.updateValueJSON))
	r.Post("/value/", newHandler(app.retrieveValueJSON))
	r.Get("/", newHandler(app.listMetrics))
	return app
}
