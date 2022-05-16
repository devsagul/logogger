package reporter

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"logogger/internal/poller"
	"logogger/internal/schema"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestReportMetrics(t *testing.T) {
	p, _ := poller.Poller(0)
	m := p()

	reported := map[string]bool{}
	rMu := sync.Mutex{}

	// fill the reported set by inspecting Metrics struct
	reflected := reflect.ValueOf(poller.Metrics{}).Type()
	for i := 0; i < reflected.NumField(); i++ {
		metricsField := reflected.Field(i).Name
		reported[metricsField] = false
	}

	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		url := request.URL.String()

		assert.Equal(t, "/update", url)

		var m schema.Metrics
		err := json.NewDecoder(request.Body).Decode(&m)

		assert.Nil(t, err)

		tField := m.MType
		name := m.ID

		rMu.Lock()
		defer rMu.Unlock()

		reportedTwice, ok := reported[name]
		assert.True(t, ok)
		assert.False(t, reportedTwice)
		reported[name] = true

		field, ok := reflected.FieldByName(name)
		assert.True(t, ok)
		fieldType := strings.ToLower(field.Type.Name())
		assert.Equal(t, tField, fieldType)

		switch tField {
		case "counter":
			assert.NotNil(t, m.Delta)
		case "gauge":
			assert.NotNil(t, m.Value)
		default:
			t.Fatalf("Unknown metrics type %s", tField)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()
	err := ReportMetrics(m, server.URL)

	assert.Nil(t, err)

	rMu.Lock()
	defer rMu.Unlock()
	for _, value := range reported {
		assert.True(t, value)
	}
}

func TestReportMetrics_FaultyServer(t *testing.T) {
	p, _ := poller.Poller(0)
	m := p()
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		url := request.URL
		fmt.Println(url)
		writer.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(handler)

	err := ReportMetrics(m, server.URL)
	server.Close()
	err2 := ReportMetrics(m, server.URL)

	assert.NotNil(t, err)
	assert.NotNil(t, err2)
}
