package server

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"logogger/internal/storage"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type counterResponse struct {
	value int64
	found bool
	err   error
}

type gaugeResponse struct {
	value float64
	found bool
	err   error
}

type listResponse struct {
	list []storage.MetricDef
	err  error
}

type App struct {
	store  storage.MetricsStorage
	Router *chi.Mux
}

func (app App) getValue(w http.ResponseWriter, r *http.Request) {
	valueType := chi.URLParam(r, "Type")
	name := chi.URLParam(r, "Name")

	ctx := r.Context()

	if valueType == "counter" {
		read := make(chan counterResponse)
		go func() {
			value, found, err := app.store.GetCounter(name)
			read <- counterResponse{value, found, err}
		}()
		select {
		case response := <-read:
			if response.err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				body := "Internal Server Error"
				_, _ = w.Write([]byte(body))
				return
			} else if !response.found {
				w.WriteHeader(http.StatusNotFound)
				body := "NotFound"
				_, _ = w.Write([]byte(body))
				return
			} else {
				w.WriteHeader(http.StatusOK)
				body := fmt.Sprintf("%d", response.value)
				_, _ = w.Write([]byte(body))
				return
			}
		case <-ctx.Done():
			return
		}
	} else if valueType == "gauge" {
		read := make(chan gaugeResponse)
		go func() {
			value, found, err := app.store.GetGauge(name)
			read <- gaugeResponse{value, found, err}
		}()
		select {
		case response := <-read:
			if response.err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				body := "Internal Server Error"
				_, _ = w.Write([]byte(body))
				return
			} else if !response.found {
				w.WriteHeader(http.StatusNotFound)
				body := "NotFound"
				_, _ = w.Write([]byte(body))
				return
			} else {
				w.WriteHeader(http.StatusOK)
				body := fmt.Sprintf("%f", response.value)
				_, _ = w.Write([]byte(body))
				return
			}
		case <-ctx.Done():
			return
		}
	} else {
		w.WriteHeader(http.StatusNotImplemented)
		body := fmt.Sprintf("Status: ERROR\nUnknown metric type %s", name)
		_, _ = w.Write([]byte(body))
		return
	}
}

func (app App) updateValue(w http.ResponseWriter, r *http.Request) {
	valueType := chi.URLParam(r, "Type")
	name := chi.URLParam(r, "Name")
	rawValue := chi.URLParam(r, "Value")

	ctx := r.Context()

	if valueType == "counter" {
		write := make(chan error)
		value, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			body := fmt.Sprintf("Status: ERROR\nCouldn't parse float from %s", rawValue)
			_, _ = w.Write([]byte(body))
			return
		}
		go func() {
			err := app.store.IncrementCounter(name, value)
			write <- err
		}()
		select {
		case err := <-write:
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				body := "Internal Server Error"
				_, _ = w.Write([]byte(body))
				return
			} else {
				w.WriteHeader(http.StatusOK)
				body := "Status: OK"
				_, _ = w.Write([]byte(body))
				return
			}
		case <-ctx.Done():
			return
		}
	} else if valueType == "gauge" {
		write := make(chan error)
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			body := fmt.Sprintf("Status: ERROR\nCouldn't parse float from %s", rawValue)
			_, _ = w.Write([]byte(body))
			return
		}
		go func() {
			err := app.store.SetGauge(name, value)
			write <- err
		}()
		select {
		case err := <-write:
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				body := "Internal Server Error"
				_, _ = w.Write([]byte(body))
				return
			} else {
				w.WriteHeader(http.StatusOK)
				body := "Status: Ok"
				_, _ = w.Write([]byte(body))
				return
			}
		case <-ctx.Done():
			return
		}
	} else {
		w.WriteHeader(http.StatusNotImplemented)
		body := fmt.Sprintf("Status: ERROR\nUnknown metric type %s", name)
		_, _ = w.Write([]byte(body))
		return
	}
}

func (app App) listMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ch := make(chan listResponse)
	go func() {
		list, err := app.store.List()
		ch <- listResponse{list, err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			body := "Internal Server Error"
			_, _ = w.Write([]byte(body))
			return
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			var sb strings.Builder

			header := "<table><tr><th>Type</th><th>Name</th></tr>"
			sb.Write([]byte(header))

			for _, def := range res.list {
				t := def.Type
				n := def.Name
				s := fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", t, n)
				sb.Write([]byte(s))
			}

			footer := "</table>"
			sb.Write([]byte(footer))

			body := sb.String()
			_, _ = w.Write([]byte(body))
			return
		}
	case <-ctx.Done():
		return
	}
}

func NewApp() *App {
	app := new(App)
	r := chi.NewRouter()
	app.Router = r
	app.store = storage.NewMemStorage()

	// полезные мидлвари
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.SetHeader("Content-Type", "text/plain"))

	r.Get("/update/{Type}/{Name}", app.getValue)
	r.Post("/update/{Type}/{Name}/{Value}", app.updateValue)
	r.Get("/", app.listMetrics)
	return app
}
