package server

import (
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"logogger/internal/schema"
	"logogger/internal/storage"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApp_RetrieveValue(t *testing.T) {
	store := storage.NewMemStorage()
	_ = store.Put(schema.NewCounter("ctrID", 42))
	app := NewApp(store)

	params := []struct {
		t     string
		id    string
		code  int
		body  string
		exact bool
	}{
		{"counter", "ctrID", http.StatusOK, "42", true},
		{"gauge", "ctrID", http.StatusConflict, "actual type in storage is counter", false},
		{"counter", "nonExistent", http.StatusNotFound, "Could not find metrics", false},
		{"stats", "nonExistent", http.StatusNotImplemented, "Could not perform requested operation", false},
	}
	for _, param := range params {
		url := fmt.Sprintf("/value/%s/%s", param.t, param.id)
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)
		responseCode := recorder.Code
		body := recorder.Body.String()

		assert.Equal(t, param.code, responseCode)
		if param.exact {
			assert.Equal(t, param.body, body)
		} else {
			assert.Contains(t, body, param.body)
		}
	}
}

func TestApp_UpdateValue(t *testing.T) {
	store := storage.NewMemStorage()
	_ = store.Put(schema.NewGauge("ggID", 13.37))
	_ = store.Put(schema.NewCounter("ctrID", 42))
	app := NewApp(store)

	params := []struct {
		t     string
		id    string
		v     string
		value float64
		delta int64
	}{
		{"gauge", "ggID", "4.99", 4.99, 0},
		{"gauge", "newGaugeId", "13.37", 13.37, 0},
		{"counter", "cnrID", "1", 0, 1},
		{"counter", "newCnrID", "1337", 0, 1337},
	}
	for _, param := range params {
		url := fmt.Sprintf("/update/%s/%s/%s", param.t, param.id, param.v)
		req, _ := http.NewRequest(http.MethodPost, url, nil)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)
		responseCode := recorder.Code
		body := recorder.Body.String()
		if param.t == "gauge" {
			stored, _ := store.Extract(schema.NewGaugeRequest(param.id))
			actual := *stored.Value
			assert.Equal(t, param.value, actual)
		} else {
			stored, _ := store.Extract(schema.NewCounterRequest(param.id))
			actual := *stored.Delta
			assert.Equal(t, param.delta, actual)
		}

		assert.Equal(t, http.StatusOK, responseCode)
		assert.Equal(t, "Status: OK", body)
	}
}

func TestApp_UpdateValueWrongType(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)

	url := "/update/stats/ID/42"
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	recorder := httptest.NewRecorder()
	app.Router.ServeHTTP(recorder, req)
	responseCode := recorder.Code
	body := recorder.Body.String()

	assert.Equal(t, http.StatusNotImplemented, responseCode)
	assert.Contains(t, body, "Could not perform requested operation")
}

func TestApp_ListMetricsEmpty(t *testing.T) {
	app := NewApp(storage.NewMemStorage())
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	app.Router.ServeHTTP(recorder, req)
	responseCode := recorder.Code
	body := recorder.Body.String()

	assert.Equal(t, "", body)
	assert.Equal(t, http.StatusOK, responseCode)
}

func TestApp_ListMetrics(t *testing.T) {
	store := storage.NewMemStorage()
	_ = store.Put(schema.NewCounter("ctrID", 42))
	_ = store.Put(schema.NewGauge("ggID", 13.37))
	app := NewApp(store)
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	app.Router.ServeHTTP(recorder, req)
	responseCode := recorder.Code
	body := recorder.Body.String()

	assert.Contains(t, body, "counter")
	assert.Contains(t, body, "ctrID")
	assert.Contains(t, body, "42")
	assert.Contains(t, body, "gauge")
	assert.Contains(t, body, "ggID")
	assert.Contains(t, body, "13.37")
	assert.Equal(t, http.StatusOK, responseCode)
}

// faultyStorage will return a generic error upon any interaction.
// It is useful to have one for tests.
type faultyStorage struct{}

func (faultyStorage) Put(req schema.Metrics) error {
	return errors.New("generic error")
}

func (faultyStorage) Extract(req schema.Metrics) (schema.Metrics, error) {
	return schema.NewEmptyMetrics(), errors.New("generic error")
}

func (faultyStorage) Increment(req schema.Metrics, value int64) error {
	return errors.New("generic error")
}

func (faultyStorage) List() ([]schema.Metrics, error) {
	return []schema.Metrics{}, errors.New("generic error")
}

func TestApp_FaultyStorage(t *testing.T) {
	store := faultyStorage{}
	app := NewApp(store)

	params := []struct {
		url    string
		method string
	}{
		{"/", http.MethodGet},
		{"/value/counter/cntId", http.MethodGet},
		{"/update/counter/cntId/1", http.MethodPost},
	}
	for _, param := range params {
		req, _ := http.NewRequest(param.method, param.url, nil)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)

		responseCode := recorder.Code
		body := recorder.Body.String()

		assert.Equal(t, http.StatusInternalServerError, responseCode)
		assert.Equal(t, "Internal Server Error", body)
	}
}
