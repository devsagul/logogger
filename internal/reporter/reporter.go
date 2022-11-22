// Package reporter implements agent-side logic for sending reports to server
package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"logogger/internal/crypt"
	"logogger/internal/schema"
	"logogger/internal/utils"
)

type ServerResponse struct {
	resp *http.Response
	err  error
	url  string
	dur  time.Duration
}

type Poller struct {
	batches   bool
	wg        sync.WaitGroup
	encryptor crypt.Encryptor
}

func (poller *Poller) ReportMetrics(l []schema.Metrics, host string) error {
	poller.wg.Add(1)
	defer poller.wg.Done()

	eg := &errgroup.Group{}

	for _, m := range l {
		m := m
		url := fmt.Sprintf("%s/update/", host)
		eg.Go(utils.WrapGoroutinePanic(func() error {
			return postSingleRequest(url, m, poller.encryptor)
		}))
	}

	return eg.Wait()
}

func (poller *Poller) ReportMetricsBatches(l []schema.Metrics, host string) error {
	poller.wg.Add(1)
	defer poller.wg.Done()

	if !poller.batches {
		return poller.ReportMetrics(l, host)
	}

	if len(l) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s/updates/", host)
	code, err := postBatchRequest(url, l, poller.encryptor)

	// if batches url is unavailable, we should use ordinary API
	if code != http.StatusNotFound {
		return err
	}
	poller.batches = false
	return poller.ReportMetrics(l, host)
}

func (poller *Poller) Shutdown() {
	poller.wg.Wait()
}

func NewPoller(encryptor crypt.Encryptor) *Poller {
	return &Poller{batches: true, encryptor: encryptor, wg: sync.WaitGroup{}}
}

func postSingleRequest(url string, m schema.Metrics, encryptor crypt.Encryptor) error {
	data, err := json.Marshal(&m)
	if err != nil {
		return err
	}

	_, err = postRequest(context.TODO(), url, data, encryptor, map[string]string{
		"Content-Type": "application/json; charset=UTF-8",
	})
	return err
}

func postBatchRequest(url string, l []schema.Metrics, encryptor crypt.Encryptor) (int, error) {
	data, err := json.Marshal(&l)
	if err != nil {
		return 0, err
	}

	return postRequest(context.TODO(), url, data, encryptor, map[string]string{
		"Content-Type":     "application/json; charset=UTF-8",
		"Content-Encoding": "gzip",
		"Accept-Encoding":  "gzip",
	})
}

func postRequest(ctx context.Context, url string, data []byte, encryptor crypt.Encryptor, headers map[string]string) (int, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return 0, err
	}
	log.Printf("%s, Sending post request to %s", id, url)

	body, err := encryptor.Encrypt(data)
	if err != nil {
		return 0, err
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return 0, err
	}

	for key, value := range headers {
		request.Header.Set(key, value)
	}

	client := &http.Client{}

	start := time.Now()
	resp, err := client.Do(request)
	dur := time.Since(start)

	if err != nil {
		log.Printf("%s Got error after %dms: %s", id, dur.Milliseconds(), err.Error())
		return 0, err
	}

	err = resp.Body.Close()
	log.Printf("Got response after %dms", dur.Milliseconds())
	code := resp.StatusCode
	if err == nil {
		if code != http.StatusOK {
			return code, fmt.Errorf("%s server returned %d code", id, code)
		}
	}
	return code, nil
}
