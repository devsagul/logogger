package reporter

import (
	"fmt"
	"logogger/internal/poller"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type ServerResponse struct {
	url  string
	resp *http.Response
	err  error
	dur  time.Duration
}

func PostRequest(url string, ch chan<- ServerResponse) {
	start := time.Now()
	resp, err := http.Post(url, "text/plain", nil)
	dur := time.Since(start)
	if err == nil {
		ch <- ServerResponse{url, resp, err, dur}
	}
	_ = resp.Body.Close()
}

func ReportMetrics(m poller.Metrics, host string, ch chan<- ServerResponse) {
	reflected := reflect.ValueOf(m)
	for i := 0; i < reflected.NumField(); i++ {
		metricsField := reflected.Type().Field(i).Name
		metricsValue := reflected.Field(i).Interface()
		metricsType := strings.ToLower(reflected.Type().Field(i).Type.Name())
		var formatString string
		var url string
		if metricsType == "gauge" {
			formatString = "http://%s/update/%s/%s/%0.6f"
			url = fmt.Sprintf(formatString, host, metricsType, metricsField, metricsValue)
		} else {
			formatString = "http://%s/update/%s/%s/%d"
			url = fmt.Sprintf(formatString, host, metricsType, metricsField, metricsValue)
		}
		go PostRequest(url, ch)
	}
}
