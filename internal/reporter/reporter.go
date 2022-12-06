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

type Reporter struct {
	batches   bool
	wg        sync.WaitGroup
	encryptor crypt.Encryptor
	ip	      string
}

func (reporter *Reporter) ReportMetrics(ctx context.Context, l []schema.Metrics, host string) error {
	reporter.wg.Add(1)
	defer reporter.wg.Done()

	eg := &errgroup.Group{}

	for _, m := range l {
		m := m
		url := fmt.Sprintf("%s/update/", host)
		eg.Go(utils.WrapGoroutinePanic(func() error {
			return postSingleRequest(ctx, url, m, reporter.encryptor, reportet.ip)
		}))
	}

	return eg.Wait()
}

func (reporter *Reporter) ReportMetricsBatches(ctx context.Context, l []schema.Metrics, host string) error {
	reporter.wg.Add(1)
	defer reporter.wg.Done()

	if !reporter.batches {
		return reporter.ReportMetrics(ctx, l, host)
	}

	if len(l) == 0 {
		return nil
	}
	url := fmt.Sprintf("%s/updates/", host)
	code, err := postBatchRequest(ctx, url, l, reporter.encryptor, reporter.ip)

	// if batches url is unavailable, we should use ordinary API
	if code != http.StatusNotFound {
		return err
	}
	reporter.batches = false
	return reporter.ReportMetrics(ctx, l, host)
}

func (reporter *Reporter) Shutdown() {
	reporter.wg.Wait()
}

func NewReporter(encryptor crypt.Encryptor) (*Reporter, error) {
	ip, err := getgetRealIP()
	if err != nil {
		return nil, err
	}
	return &Reporter{batches: true, encryptor: encryptor, wg: sync.WaitGroup{}, ip}
}

func postSingleRequest(ctx context.Context, url string, m schema.Metrics, encryptor crypt.Encryptor, ip string) error {
	data, err := json.Marshal(&m)
	if err != nil {
		return err
	}

	_, err = postRequest(ctx, url, data, encryptor, map[string]string{
		"Content-Type": "application/json; charset=UTF-8",
	})
	return err
}

func postBatchRequest(ctx context.Context, url string, l []schema.Metrics, encryptor crypt.Encryptor, ip string) (int, error) {
	data, err := json.Marshal(&l)
	if err != nil {
		return 0, err
	}

	return postRequest(ctx, url, data, encryptor, map[string]string{
		"Content-Type":     "application/json; charset=UTF-8",
		"Content-Encoding": "gzip",
		"Accept-Encoding":  "gzip",
	})
}

func postRequest(ctx context.Context, url string, data []byte, encryptor crypt.Encryptor, headers map[string]string, ip string) (int, error) {
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

	request.Header.Set("X-Real-IP", ip)

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

func getgetRealIP() (string, err) {
	addrs, err := net.InterfaceAddrs()
    if err != nil {
        return "", err
    }
    for _, address := range addrs {
        if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			return ipnet.IP.String(), nil
        }
    }
    return "", errors.New("unable to obtain IP address")
} 
