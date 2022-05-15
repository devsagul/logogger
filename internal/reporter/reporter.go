package reporter

import (
	"fmt"
	"golang.org/x/sync/errgroup"
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

func postRequest(url string) error {
	log.Printf("Sending metrics to %s", url)
	start := time.Now()
	resp, err := http.Post(url, "text/plain", nil)
	dur := time.Since(start)
	if err != nil {
		log.Printf("Got error after %dms", dur.Milliseconds())
		return err
	}
	err = resp.Body.Close()
	log.Printf("Got response after %dms", dur.Milliseconds())
	if err == nil {
		code := resp.StatusCode
		if code >= 400 {
			return fmt.Errorf("Server returned %d code", code)
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
		var formatString string
		var url string
		if metricsType == "gauge" {
			formatString = "%s/update/%s/%s/%f"
			url = fmt.Sprintf(formatString, host, metricsType, metricsField, metricsValue)
		} else {
			formatString = "%s/update/%s/%s/%d"
			url = fmt.Sprintf(formatString, host, metricsType, metricsField, metricsValue)
		}

		eg.Go(func() error {
			return postRequest(url)
		})
	}
	return eg.Wait()
}
