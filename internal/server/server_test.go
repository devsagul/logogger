package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
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

func TestApp_UpdateValueJsonNoInput(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)

	req, _ := http.NewRequest(http.MethodPost, "/update/", nil)
	recorder := httptest.NewRecorder()
	app.Router.ServeHTTP(recorder, req)

	responseCode := recorder.Code
	body := recorder.Body.String()

	assert.Equal(t, http.StatusBadRequest, responseCode)
	assert.Contains(t, body, "Validation errors")
}

func TestApp_UpdateValueJsonInvalidInput(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)

	tests := [...]string{
		"",
		`{"id": "cntID", "type": "counter", "deltaz": 42}`,
		`{"id": "cntID", "type": "counter", "delta": 13.37}`,
		`{"id": "cntID", "type": "counter"}`,
		`{"id": "cntID", "type": "counter", "value": 13.37}`,
		`{"id": "ggID", "type": "gauge", "delta": 42}`,
	}

	for _, data := range tests {
		body := bytes.NewBufferString(data)
		req, _ := http.NewRequest(http.MethodPost, "/update/", body)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)

		responseCode := recorder.Code
		respBody := recorder.Body.String()

		assert.Equal(t, http.StatusBadRequest, responseCode)
		assert.Contains(t, respBody, "Validation errors")
	}
}

func TestApp_UpdateValueJson(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)

	params := [...]struct {
		data     schema.Metrics
		expected schema.Metrics
	}{
		{
			schema.NewCounter("cntID", 42),
			schema.NewCounter("cntID", 42),
		},
		{
			schema.NewCounter("cntID", 1),
			schema.NewCounter("cntID", 43),
		},
		{
			schema.NewGauge("ggID", 13.37),
			schema.NewGauge("cntID", 13.37),
		},
		{
			schema.NewGauge("ggID", 42),
			schema.NewGauge("cntID", 42),
		},
	}

	for _, param := range params {
		serialized, _ := json.Marshal(param.data)
		body := bytes.NewBuffer(serialized)
		req, _ := http.NewRequest(http.MethodPost, "/update/", body)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)

		responseCode := recorder.Code

		expected := param.expected
		stored, _ := store.Extract(expected)
		contentType := recorder.Header().Get("Content-Type")

		assert.Equal(t, http.StatusOK, responseCode)
		assert.Equal(t, expected.ID, stored.ID)
		assert.Equal(t, expected.MType, stored.MType)
		switch expected.MType {
		case "counter":
			assert.Equal(t, *expected.Delta, *stored.Delta)
		case "gauge":
			assert.Equal(t, *expected.Value, *stored.Value)
		}
		assert.Equal(t, "application/json", contentType)
	}
}

func TestApp_UpdateValueJSON_WrongType(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)
	d := int64(42)
	f := 13.37
	m := schema.Metrics{
		ID: "id", MType: "statistics", Delta: &d, Value: &f,
	}

	serialized, _ := json.Marshal(m)
	body := bytes.NewBuffer(serialized)
	req, _ := http.NewRequest(http.MethodPost, "/update/", body)
	recorder := httptest.NewRecorder()
	app.Router.ServeHTTP(recorder, req)

	responseCode := recorder.Code
	respBody := recorder.Body.String()

	assert.Equal(t, http.StatusNotImplemented, responseCode)
	assert.Contains(t, respBody, "Could not perform requested operation")
}

func TestApp_RetrieveValueJSONNoInput(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)

	req, _ := http.NewRequest(http.MethodPost, "/value/", nil)
	recorder := httptest.NewRecorder()
	app.Router.ServeHTTP(recorder, req)

	responseCode := recorder.Code
	body := recorder.Body.String()

	assert.Equal(t, http.StatusBadRequest, responseCode)
	assert.Contains(t, body, "Validation errors")
}

func TestApp_RetrieveValueJSONInvalidInput(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)

	tests := [...]string{
		"",
		`{"id": "cntID", "type": "counter", "deltaz": 42}`,
		`{"id": "cntID", "type": "counter", "delta": 13.37}`,
	}

	for _, data := range tests {
		body := bytes.NewBufferString(data)
		req, _ := http.NewRequest(http.MethodPost, "/value/", body)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)

		responseCode := recorder.Code
		respBody := recorder.Body.String()

		assert.Equal(t, http.StatusBadRequest, responseCode)
		assert.Contains(t, respBody, "Validation errors")
	}
}

func TestApp_RetrieveValueJSON(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)

	m := schema.NewCounter("ctrID", 42)
	marshalled, _ := json.Marshal(m)
	_ = store.Put(m)

	params := [...]struct {
		request schema.Metrics
		code    int
		needle  string
	}{
		{schema.NewCounterRequest("nonExistent"), http.StatusNotFound, "Could not find"},
		{schema.NewCounterRequest("ctrID"), http.StatusOK, string(marshalled)},
		{schema.NewGaugeRequest("ctrID"), http.StatusConflict, "actual type in storage is counter"},
	}

	for _, param := range params {
		serialized, _ := json.Marshal(param.request)
		body := bytes.NewBuffer(serialized)
		req, _ := http.NewRequest(http.MethodPost, "/value/", body)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)

		responseCode := recorder.Code
		respBody := recorder.Body.String()

		assert.Equal(t, param.code, responseCode)
		assert.Contains(t, respBody, param.needle)
	}
}

func TestApp_RetrieveValueJSONWrongType(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)
	m := schema.Metrics{
		ID: "id", MType: "statistics",
	}

	serialized, _ := json.Marshal(m)
	body := bytes.NewBuffer(serialized)
	req, _ := http.NewRequest(http.MethodPost, "/value/", body)
	recorder := httptest.NewRecorder()
	app.Router.ServeHTTP(recorder, req)

	responseCode := recorder.Code
	respBody := recorder.Body.String()

	assert.Equal(t, http.StatusNotImplemented, responseCode)
	assert.Contains(t, respBody, "Could not perform requested operation")
}

func TestApp_FaultyStorage(t *testing.T) {
	store := faultyStorage{}
	app := NewApp(store)

	params := []struct {
		url    string
		method string
		body   string
	}{
		{"/", http.MethodGet, ""},
		{"/update/", http.MethodPost, `{"id": "cntID", "type": "counter", "delta": 42}`},
		{"/value/", http.MethodPost, `{"id": "cntID", "type": "counter"}`},
		{"/update/counter/cntId/1", http.MethodPost, ""},
		{"/value/counter/cntId", http.MethodGet, ""},
	}
	for _, param := range params {
		var reqBody io.Reader
		if param.body == "" {
			reqBody = nil
		} else {
			reqBody = bytes.NewBufferString(param.body)
		}
		req, _ := http.NewRequest(param.method, param.url, reqBody)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)

		responseCode := recorder.Code
		body := recorder.Body.String()

		assert.Equal(t, http.StatusInternalServerError, responseCode)
		assert.Contains(t, body, "Internal Server Error")
	}
}

/**
Mocks used for specific tests
*/

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
