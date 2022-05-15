package reporter

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"logogger/internal/poller"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestReportMetrics(t *testing.T) {
	p, _ := poller.Poller(0)
	m := p()
	re := regexp.MustCompile(`/update/([^/]+)/([^/]+)/([^/]+)$`)

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
		match := re.FindStringSubmatch(url)
		assert.NotEmpty(t, match)

		fmt.Println(match)

		tField := match[1]
		name := match[2]
		val := match[3]

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
			_, err := strconv.ParseInt(val, 10, 64)
			assert.Nil(t, err)
		case "gauge":
			_, err := strconv.ParseFloat(val, 64)
			assert.Nil(t, err)
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
