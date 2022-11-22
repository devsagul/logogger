// Package reporter implements agent-side logic for sending reports to server
package reporter

import (
	"bytes"
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
			return postRequest(url, m, poller.encryptor)
		}))
	}

	return eg.Wait()
}

func (poller *Poller) ReportMetricsBatches(l []schema.Metrics, host string) error {
	poller.wg.Add(1)
	defer poller.wg.Done()

	if poller.batches {
		if len(l) == 0 {
			return nil
		}
		url := fmt.Sprintf("%s/updates/", host)
		code, err := postBatchRequest(url, l, poller.encryptor)
		if code == http.StatusNotFound {
			// if the server can't handle /updates URL, we should
			// use standard handle
			poller.batches = false
		} else {
			return err
		}
	}
	return poller.ReportMetrics(l, host)
}

func (poller *Poller) Shutdown() {
	poller.wg.Wait()
}

func NewPoller(encryptor crypt.Encryptor) *Poller {
	return &Poller{batches: true, encryptor: encryptor, wg: sync.WaitGroup{}}
}

func postRequest(url string, m schema.Metrics, encryptor crypt.Encryptor) error {
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

	b, err = encryptor.Encrypt(b)
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
		if code != http.StatusOK {
			return fmt.Errorf("%s server returned %d code", id, code)
		}
	}
	return err
}

func postBatchRequest(url string, l []schema.Metrics, encryptor crypt.Encryptor) (int, error) {
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

	b, err = encryptor.Encrypt(b)
	if err != nil {
		return 0, err
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("Content-Encoding", "gzip")
	request.Header.Set("Accept-Encoding", "gzip")

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
		if code != http.StatusOK {
			return code, fmt.Errorf("%s server returned %d code", id, code)
		}
	}
	return code, nil
}
