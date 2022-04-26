package server

import (
	"fmt"
	"logogger/internal/poller"
	"logogger/internal/router"
	"logogger/internal/storage"
	"net/http"
	"reflect"
	"strconv"
)

type gauge struct {
	name  string
	value float64
}

type counter struct {
	name  string
	value int64
}

type App struct {
	store     storage.MetricsStorage
	r         router.Router
	set       chan gauge
	increment chan counter
	write     chan poller.Metrics
}

func (app App) handleGauge(w http.ResponseWriter, r *http.Request, args []string) {
	name := args[0]
	metrics, _ := app.store.Read()
	reflected := reflect.ValueOf(metrics)
	if !reflected.Elem().FieldByName(name).IsValid() {
		w.WriteHeader(http.StatusNotFound)
		body := "Status: ERROR\nNot Found"
		w.Write([]byte(body))
		return
	}

	rawValue := args[1]
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		body := fmt.Sprintf("Status: ERROR\nCouldn't parse float from %s", rawValue)
		w.Write([]byte(body))
		return
	}
	app.set <- gauge{name, value}
	w.WriteHeader(http.StatusOK)
	body := "Status: OK"
	w.Write([]byte(body))
	return
}

func (app App) handleCounter(w http.ResponseWriter, r *http.Request, args []string) {
	name := args[0]
	metrics, err := app.store.Read()
	reflected := reflect.ValueOf(metrics)
	if !reflected.Elem().FieldByName(name).IsValid() {
		w.WriteHeader(http.StatusNotFound)
		body := "Status: ERROR\nNot Found"
		w.Write([]byte(body))
		return
	}

	rawValue := args[1]
	value, err := strconv.ParseInt(rawValue, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		body := fmt.Sprintf("Status: ERROR\nCouldn't parse float from %s", rawValue)
		w.Write([]byte(body))
		return
	}
	app.increment <- counter{name, value}
	w.WriteHeader(http.StatusOK)
	body := "Status: OK"
	w.Write([]byte(body))
	return
}

func (app App) WriteMetrics() {
	metrics := <-app.write
	_ = app.store.Write(metrics)
}

func (app App) SetGauge() {
	g := <-app.set
	metrics, _ := app.store.Read()
	reflected := reflect.ValueOf(&metrics)
	reflected.Elem().FieldByName(g.name).SetFloat(g.value)
	app.write <- metrics
}

func (app App) IncrementCounter() {
	g := <-app.increment
	metrics, _ := app.store.Read()
	reflected := reflect.ValueOf(&metrics)
	field := reflected.Elem().FieldByName(g.name)
	value := field.Int()
	field.SetInt(value + g.value)
	app.write <- metrics
}

func NewApp() *App {
	app := new(App)

	app.store = storage.NewMemStorage()
	app.r.RegisterHandler(`/update/gauge/(?P<Name>\w+)/(?P<Value>[^/])+`, app.handleGauge)
	app.r.RegisterHandler(`/update/counter/(?P<Name>\w+)/(?P<Value>)[^/]+`, app.handleCounter)
	go app.WriteMetrics()
	go app.SetGauge()
	go app.IncrementCounter()
	return app
}

func (app App) Handle(w http.ResponseWriter, r *http.Request) {
	app.r.Handle(w, r)
}
