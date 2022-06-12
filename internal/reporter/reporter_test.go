package reporter

import (
	"encoding/json"
	"fmt"
	"logogger/internal/poller"
	"logogger/internal/schema"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReportMetrics(t *testing.T) {
	p, err := poller.NewPoller(0)
	if err != nil {
		t.Fatalf("Error accessing storage.")
	}

	l, err := p.Poll()
	if err != nil {
		t.Fatalf("Error polling data.")
	}

	reported := map[string]bool{}
	rMu := sync.Mutex{}

	// fill the reported set by inspecting Metrics struct
	for _, m := range poller.SysMetrics {
		reported[m] = false
	}
	reported["RandomValue"] = false
	reported["PollCount"] = false

	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		url := request.URL.String()

		assert.Equal(t, "/update/", url)

		var m schema.Metrics
		err := json.NewDecoder(request.Body).Decode(&m)

		assert.Nil(t, err)
		if err != nil {
			t.Fatalf("Error decoding metrics.")
		}

		tField := m.MType
		name := m.ID

		rMu.Lock()
		defer rMu.Unlock()
		reportedTwice, _ := reported[name]
		assert.False(t, reportedTwice)
		reported[name] = true

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
	err = ReportMetrics(l, server.URL)

	if err != nil {
		assert.FailNow(t, "Error reporting data.")
	}

	for _, value := range reported {
		assert.True(t, value)
	}
}

func TestReportMetrics_FaultyServer(t *testing.T) {
	p, err := poller.NewPoller(0)
	if err != nil {
		assert.FailNow(t, "Error accessing storage.")
	}

	l, err := p.Poll()
	if err != nil {
		assert.FailNow(t, "Error polling data.")
	}

	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		url := request.URL
		fmt.Println(url)
		writer.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(handler)

	err1 := ReportMetrics(l, server.URL)
	server.Close()
	err2 := ReportMetrics(l, server.URL)

	assert.NotNil(t, err1)
	assert.NotNil(t, err2)
}
