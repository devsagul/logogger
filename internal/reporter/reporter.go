package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"log"
	"logogger/internal/schema"
	"net/http"
	"time"
)

type ServerResponse struct {
	url  string
	resp *http.Response
	err  error
	dur  time.Duration
}

func postRequest(url string, m schema.Metrics) error {
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	log.Printf("%s, Sending %s to %s", id, m.ID, url)
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
		log.Printf("%s Got error after %dms", id, dur.Milliseconds())
		return err
	}
	err = resp.Body.Close()
	log.Printf("Got response after %dms", dur.Milliseconds())
	if err == nil {
		code := resp.StatusCode
		if code != 200 {
			return fmt.Errorf("%s server returned %d code", id, code)
		}
	}
	return err
}

func ReportMetrics(l []schema.Metrics, host string) error {
	eg := &errgroup.Group{}

	for _, m := range l {
		m := m
		url := fmt.Sprintf("%s/update/", host)
		eg.Go(func() error {
			return postRequest(url, m)
		})
	}

	return eg.Wait()
}
