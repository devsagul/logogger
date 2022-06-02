package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"logogger/internal/schema"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type ServerResponse struct {
	url  string
	resp *http.Response
	err  error
	dur  time.Duration
}

var batches = true

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
		log.Printf("%s Got error after %dms: %s", id, dur.Milliseconds(), err.Error())
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

func postBatchRequest(url string, l []schema.Metrics) (int, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return 0, err
	}
	log.Printf("%s, Sending batch update to %s", id, url)
	start := time.Now()
	b, err := json.Marshal(&l)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	dur := time.Since(start)
	resp, err := client.Do(request)
	if err != nil {
		log.Printf("%s Got error after %dms: %s", id, dur.Milliseconds(), err.Error())
		return 0, err
	}
	err = resp.Body.Close()
	log.Printf("Got response after %dms", dur.Milliseconds())
	code := resp.StatusCode
	if err == nil {
		if code != 200 {
			return code, fmt.Errorf("%s server returned %d code", id, code)
		}
	}
	return code, nil
}

func ReportMetrics(l []schema.Metrics, host string) error {
	if batches {
		if len(l) == 0 {
			return nil
		}
		url := fmt.Sprintf("%s/updates/", host)
		code, err := postBatchRequest(url, l)
		if code == 404 {
			batches = false
			return ReportMetrics(l, host)
		}
		return err
	} else {
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
}
