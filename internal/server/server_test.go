package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"logogger/internal/schema"
	"logogger/internal/storage"
)

func TestApp_RetrieveValue(t *testing.T) {
	store := storage.NewMemStorage()
	err := store.Put(context.Background(), schema.NewCounter("ctrID", 42))
	assert.NoError(t, err)
	app := NewApp(store)

	params := []struct {
		t     schema.MetricsType
		id    string
		body  string
		code  int
		exact bool
	}{
		{schema.MetricsTypeCounter, "ctrID", "42", http.StatusOK, true},
		{schema.MetricsTypeGauge, "ctrID", "actual type in storage is counter", http.StatusConflict, false},
		{schema.MetricsTypeCounter, "nonExistent", "Could not find metrics", http.StatusNotFound, false},
		{"stats", "nonExistent", "Could not perform requested operation", http.StatusNotImplemented, false},
	}
	for _, param := range params {
		url := fmt.Sprintf("/value/%s/%s", param.t, param.id)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		assert.NoError(t, err)
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
	err := store.Put(context.Background(), schema.NewGauge("ggID", 13.37))
	assert.NoError(t, err)
	err = store.Put(context.Background(), schema.NewCounter("ctrID", 42))
	assert.NoError(t, err)
	app := NewApp(store)

	params := []struct {
		t     schema.MetricsType
		id    string
		v     string
		value float64
		delta int64
	}{
		{schema.MetricsTypeGauge, "ggID", "4.99", 4.99, 0},
		{schema.MetricsTypeGauge, "newGaugeId", "13.37", 13.37, 0},
		{schema.MetricsTypeCounter, "cnrID", "1", 0, 1},
		{schema.MetricsTypeCounter, "newCnrID", "1337", 0, 1337},
	}
	for _, param := range params {
		url := fmt.Sprintf("/update/%s/%s/%s", param.t, param.id, param.v)
		req, err := http.NewRequest(http.MethodPost, url, nil)
		assert.NoError(t, err)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)
		responseCode := recorder.Code
		body := recorder.Body.String()
		if param.t == "gauge" {
			stored, err := store.Extract(context.Background(), schema.NewGaugeRequest(param.id))
			assert.NoError(t, err)
			actual := *stored.Value
			assert.Equal(t, param.value, actual)
		} else {
			stored, err := store.Extract(context.Background(), schema.NewCounterRequest(param.id))
			assert.NoError(t, err)
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
	req, err := http.NewRequest(http.MethodPost, url, nil)
	assert.NoError(t, err)
	recorder := httptest.NewRecorder()
	app.Router.ServeHTTP(recorder, req)
	responseCode := recorder.Code
	body := recorder.Body.String()

	assert.Equal(t, http.StatusNotImplemented, responseCode)
	assert.Contains(t, body, "Could not perform requested operation")
}

func TestApp_ListMetricsEmpty(t *testing.T) {
	app := NewApp(storage.NewMemStorage())
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	assert.NoError(t, err)
	recorder := httptest.NewRecorder()

	app.Router.ServeHTTP(recorder, req)
	responseCode := recorder.Code
	body := recorder.Body.String()

	assert.Equal(t, "", body)
	assert.Equal(t, http.StatusOK, responseCode)
}

func TestApp_ListMetrics(t *testing.T) {
	store := storage.NewMemStorage()
	err := store.Put(context.Background(), schema.NewCounter("ctrID", 42))
	assert.NoError(t, err)
	err = store.Put(context.Background(), schema.NewGauge("ggID", 13.37))
	assert.NoError(t, err)
	app := NewApp(store)
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	assert.NoError(t, err)
	recorder := httptest.NewRecorder()

	app.Router.ServeHTTP(recorder, req)
	responseCode := recorder.Code
	body := recorder.Body.String()

	assert.Contains(t, body, schema.MetricsTypeCounter)
	assert.Contains(t, body, "ctrID")
	assert.Contains(t, body, "42")
	assert.Contains(t, body, schema.MetricsTypeGauge)
	assert.Contains(t, body, "ggID")
	assert.Contains(t, body, "13.37")
	assert.Equal(t, http.StatusOK, responseCode)
}

func TestApp_UpdateValueJsonNoInput(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)

	req, err := http.NewRequest(http.MethodPost, "/update/", nil)
	assert.NoError(t, err)
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
		req, err := http.NewRequest(http.MethodPost, "/update/", body)
		assert.NoError(t, err)
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
			schema.NewGauge("ggID", 13.37),
		},
		{
			schema.NewGauge("ggID", 42),
			schema.NewGauge("ggID", 42),
		},
	}

	for _, param := range params {
		serialized, err := json.Marshal(param.data)
		assert.NoError(t, err)
		body := bytes.NewBuffer(serialized)
		req, err := http.NewRequest(http.MethodPost, "/update/", body)
		assert.NoError(t, err)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)

		responseCode := recorder.Code

		expected := param.expected

		var actual schema.Metrics
		decoder := json.NewDecoder(recorder.Body)
		err = decoder.Decode(&actual)
		assert.NoError(t, err)
		contentType := recorder.Header().Get("Content-Type")

		assert.Equal(t, http.StatusOK, responseCode)
		assert.Equal(t, expected.ID, actual.ID)
		assert.Equal(t, expected.MType, actual.MType)
		switch expected.MType {
		case schema.MetricsTypeCounter:
			assert.Equal(t, *expected.Delta, *actual.Delta)
		case schema.MetricsTypeGauge:
			assert.Equal(t, *expected.Value, *actual.Value)
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

	serialized, err := json.Marshal(m)
	assert.NoError(t, err)
	body := bytes.NewBuffer(serialized)
	req, err := http.NewRequest(http.MethodPost, "/update/", body)
	assert.NoError(t, err)
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

	req, err := http.NewRequest(http.MethodPost, "/value/", nil)
	assert.NoError(t, err)
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
		req, err := http.NewRequest(http.MethodPost, "/value/", body)
		assert.NoError(t, err)
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
	marshalled, err := json.Marshal(m)
	assert.NoError(t, err)
	err = store.Put(context.Background(), m)
	assert.NoError(t, err)

	params := [...]struct {
		request schema.Metrics
		needle  string
		code    int
	}{
		{schema.NewCounterRequest("nonExistent"), "Could not find", http.StatusNotFound},
		{schema.NewCounterRequest("ctrID"), string(marshalled), http.StatusOK},
		{schema.NewGaugeRequest("ctrID"), "actual type in storage is counter", http.StatusConflict},
	}

	for _, param := range params {
		serialized, err := json.Marshal(param.request)
		assert.NoError(t, err)
		body := bytes.NewBuffer(serialized)
		req, err := http.NewRequest(http.MethodPost, "/value/", body)
		assert.NoError(t, err)
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

	serialized, err := json.Marshal(m)
	assert.NoError(t, err)
	body := bytes.NewBuffer(serialized)
	req, err := http.NewRequest(http.MethodPost, "/value/", body)
	assert.NoError(t, err)
	recorder := httptest.NewRecorder()
	app.Router.ServeHTTP(recorder, req)

	responseCode := recorder.Code
	respBody := recorder.Body.String()

	assert.Equal(t, http.StatusNotImplemented, responseCode)
	assert.Contains(t, respBody, "Could not perform requested operation")
}

func TestApp_UpdateValueJSON(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)

	m := schema.NewCounter("ctrID", 42)
	marshalled, err := json.Marshal(m)
	assert.NoError(t, err)
	err = store.Put(context.Background(), m)
	assert.NoError(t, err)

	params := [...]struct {
		request []schema.Metrics
		needle  string
		code    int
	}{
		{[]schema.Metrics{schema.NewCounterRequest("nonExistent")}, "Could not find", http.StatusNotFound},
		{[]schema.Metrics{schema.NewCounterRequest("ctrID")}, string(marshalled), http.StatusOK},
		{[]schema.Metrics{schema.NewGaugeRequest("ctrID")}, "actual type in storage is counter", http.StatusConflict},
	}

	for _, param := range params {
		serialized, err := json.Marshal(param.request)
		assert.NoError(t, err)
		body := bytes.NewBuffer(serialized)
		req, err := http.NewRequest(http.MethodPost, "/updates/", body)
		assert.NoError(t, err)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)

		responseCode := recorder.Code
		respBody := recorder.Body.String()

		assert.Equal(t, param.code, responseCode)
		assert.Contains(t, respBody, param.needle)
	}
}

func TestApp_UpdateValueJSONWrongType(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)
	m := schema.Metrics{
		ID: "id", MType: "statistics",
	}

	serialized, err := json.Marshal(m)
	assert.NoError(t, err)
	body := bytes.NewBuffer(serialized)
	req, err := http.NewRequest(http.MethodPost, "/value/", body)
	assert.NoError(t, err)
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
		req, err := http.NewRequest(param.method, param.url, reqBody)
		assert.NoError(t, err)
		recorder := httptest.NewRecorder()
		app.Router.ServeHTTP(recorder, req)

		responseCode := recorder.Code
		body := recorder.Body.String()

		assert.Equal(t, http.StatusInternalServerError, responseCode)
		assert.Contains(t, body, "Internal Server Error")
	}
}

func TestApp_Ping(t *testing.T) {
	store := storage.NewMemStorage()
	app := NewApp(store)

	req, err := http.NewRequest(http.MethodGet, "/ping", nil)
	assert.NoError(t, err)
	recorder := httptest.NewRecorder()
	app.Router.ServeHTTP(recorder, req)

	responseCode := recorder.Code

	assert.Equal(t, http.StatusOK, responseCode)
}

func TestApp_PingFaulty(t *testing.T) {
	store := faultyStorage{}
	app := NewApp(store)

	req, err := http.NewRequest(http.MethodGet, "/ping", nil)
	assert.NoError(t, err)
	recorder := httptest.NewRecorder()
	app.Router.ServeHTTP(recorder, req)

	responseCode := recorder.Code

	assert.NotEqual(t, http.StatusOK, responseCode)
}

/**
Mocks used for specific tests
*/

// faultyStorage will return a generic error upon any interaction.
// It is useful to have one for tests.
type faultyStorage struct{}

func (faultyStorage) Put(_ context.Context, req schema.Metrics) error {
	return errors.New("generic error")
}

func (faultyStorage) Extract(_ context.Context, req schema.Metrics) (schema.Metrics, error) {
	return schema.NewEmptyMetrics(), errors.New("generic error")
}

func (faultyStorage) Increment(_ context.Context, req schema.Metrics, value int64) error {
	return errors.New("generic error")
}

func (faultyStorage) List(_ context.Context) ([]schema.Metrics, error) {
	return []schema.Metrics{}, errors.New("generic error")
}

func (faultyStorage) BulkPut(_ context.Context, metrics []schema.Metrics) error {
	return errors.New("generic error")
}

func (faultyStorage) BulkUpdate(_ context.Context, counters []schema.Metrics, gauges []schema.Metrics) error {
	return errors.New("generic error")
}

func (faultyStorage) Ping(_ context.Context) error {
	return errors.New("generic error")
}

func (faultyStorage) Close() error {
	return nil
}
