package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/sync/errgroup"
	"log"
	"logogger/internal/poller"
	"logogger/internal/schema"
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

func postRequest(url string, m schema.Metrics) error {
	log.Printf("Sending metrics to %s", url)
	start := time.Now()
	b, err := json.Marshal(&m)
	if err != nil {
		return err
	}
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	dur := time.Since(start)
	resp, err := client.Do(request)
	if err != nil {
		log.Printf("Got error after %dms", dur.Milliseconds())
		return err
	}
	err = resp.Body.Close()
	log.Printf("Got response after %dms", dur.Milliseconds())
	if err == nil {
		code := resp.StatusCode
		if code >= 400 {
			return fmt.Errorf("server returned %d code", code)
		}
	}
	return err
}

func ReportMetrics(m poller.Metrics, host string) error {
	reflected := reflect.ValueOf(m)
	eg := &errgroup.Group{}

	for i := 0; i < reflected.NumField(); i++ {
		metricsField := reflected.Type().Field(i).Name
		metricsValue := reflected.Field(i).Interface()
		metricsType := strings.ToLower(reflected.Type().Field(i).Type.Name())

		url := fmt.Sprintf("%s/update/", host)

		var m schema.Metrics
		if metricsType == "gauge" {
			m = schema.NewGauge(metricsField, float64(metricsValue.(poller.Gauge)))
		} else {
			m = schema.NewCounter(metricsField, int64(metricsValue.(poller.Counter)))
		}

		eg.Go(func() error {
			return postRequest(url, m)
		})
	}
	return eg.Wait()
}
