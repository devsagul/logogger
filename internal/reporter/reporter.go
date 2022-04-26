package reporter

import (
	"fmt"
	"log"
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

func PostRequest(url string) {
	log.Printf("Sending metrics to %s", url)
	start := time.Now()
	resp, err := http.Post(url, "text/plain", nil)
	dur := time.Since(start)
	if err != nil {
		log.Printf("Got error after %dms", dur.Milliseconds())
	} else {
		_ = resp.Body.Close()
		log.Printf("Got response after %dms", dur.Milliseconds())
	}
}

func ReportMetrics(m poller.Metrics, host string) {
	reflected := reflect.ValueOf(m)
	for i := 0; i < reflected.NumField(); i++ {
		metricsField := reflected.Type().Field(i).Name
		metricsValue := reflected.Field(i).Interface()
		metricsType := strings.ToLower(reflected.Type().Field(i).Type.Name())
		var formatString string
		var url string
		if metricsType == "gauge" {
			formatString = "http://%s/update/%s/%s/%f"
			url = fmt.Sprintf(formatString, host, metricsType, metricsField, metricsValue)
		} else {
			formatString = "http://%s/update/%s/%s/%d"
			url = fmt.Sprintf(formatString, host, metricsType, metricsField, metricsValue)
		}
		go PostRequest(url)
	}
}
