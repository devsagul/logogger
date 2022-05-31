package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"logogger/internal/dumper"
	"logogger/internal/schema"
	"logogger/internal/storage"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type App struct {
	store  storage.MetricsStorage
	Router *chi.Mux
	sync   bool
	dumper dumper.Dumper
	key    string
}

type errorHTTPHandler func(http.ResponseWriter, *http.Request) error

func newHandler(handler errorHTTPHandler) http.HandlerFunc {
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
	if app.sync {
		app.safeDump()
	}
	return nil
}

func (app App) listMetrics(w http.ResponseWriter, _ *http.Request) error {
	list, err := app.store.List()
	if err != nil {
		return err
	}

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

	if app.key != "" {
		signed, err := m.IsSignedWithKey(app.key)
		if err != nil {
			return err
		}
		if !signed {
			return ValidationError("signature mismatch")
		}
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

	if app.key != "" {
		if err = value.Sign(app.key); err != nil {
			return err
		}
	}

	serialized, err := json.Marshal(value)
	if err != nil {
		return err
	}

	SafeWrite(w, http.StatusOK, string(serialized))
	if app.sync {
		go app.safeDump()
	}
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
	case "gauge":
	default:
		return &requestError{
			status: http.StatusNotImplemented,
			body:   fmt.Sprintf("Could not perform requested operation on metric type %s", m.MType),
		}
	}

	value, err = app.store.Extract(m)
	if err != nil {
		return err
	}

	if app.key != "" {
		err = value.Sign(app.key)
		if err != nil {
			return err
		}
	}

	serialized, err := json.Marshal(value)
	if err != nil {
		return err
	}

	SafeWrite(w, http.StatusOK, string(serialized))
	return nil
}

func (app App) ping(w http.ResponseWriter, r *http.Request) error {
	err := app.store.Ping()
	if err != nil {
		return err
	}
	SafeWrite(w, http.StatusOK, "Status: OK")
	return nil
}

func (app *App) safeDump() {
	log.Print("Dumping current storage state...")
	l, err := app.store.List()
	if err != nil {
		log.Print("Could not retrieve values from storage")
		return
	}

	err = app.dumper.Dump(l)
	if err != nil {
		log.Print("Could not write storage data")
	}
}

func NewApp(
	store storage.MetricsStorage,
) *App {
	app := new(App)
	r := chi.NewRouter()
	app.Router = r
	app.store = store
	app.dumper = dumper.NoOpDumper{}
	app.sync = false
	app.key = ""

	// useful middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.Compress(5))

	r.With(middleware.SetHeader("Content-Type", "text/plain")).Post("/update/{Type}/{Name}/{Value}", newHandler(app.updateValue))
	r.With(middleware.SetHeader("Content-Type", "text/plain")).Get("/value/{Type}/{Name}", newHandler(app.retrieveValue))
	r.With(middleware.SetHeader("Content-Type", "application/json")).Post("/update/", newHandler(app.updateValueJSON))
	r.With(middleware.SetHeader("Content-Type", "application/json")).Post("/value/", newHandler(app.retrieveValueJSON))
	r.With(middleware.SetHeader("Content-Type", "text/plain")).Get("/ping", newHandler(app.ping))
	r.With(middleware.SetHeader("Content-Type", "text/html")).Get("/", newHandler(app.listMetrics))
	return app
}

func (app *App) WithDumper(d dumper.Dumper) *App {
	app.dumper = d
	return app
}

func (app *App) WithDumpInterval(interval time.Duration) *App {
	if interval == 0 {
		app.sync = true
		return app
	}

	app.sync = false
	t := time.NewTicker(interval)
	go func() {
		for {
			<-t.C
			app.safeDump()
		}
	}()

	return app
}

func (app *App) WithKey(key string) *App {
	app.key = key
	return app
}
