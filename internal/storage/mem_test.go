package storage

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"logogger/internal/schema"
	"sync"
	"testing"
)

const concurrency = 50

func TestMemStorage_PutSingle(t *testing.T) {
	storage := NewMemStorage()
	err := storage.Put(schema.NewCounter("counter", 42))
	assert.Equal(t, err, nil)
}

func TestMemStorage_PutTypeMismatch(t *testing.T) {
	storage := NewMemStorage()
	_ = storage.Put(schema.NewCounter("generic", 42))

	err := storage.Put(schema.NewGauge("generic", 13.37))

	assert.Errorf(t, err, "Did not return error on mismatched types")
	assert.IsType(t, &TypeMismatch{}, err)
}

func TestMemStorage_PutConcurrentSameKey(t *testing.T) {
	// arrange
	storage := NewMemStorage()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	start := make(chan struct{})
	go func() {
		<-start
		wg.Done()
	}()

	eg := &errgroup.Group{}
	for i := 0; i < concurrency; i++ {
		i := i
		eg.Go(func() error {
			wg.Wait()
			return storage.Put(schema.NewCounter("counter", int64(i)))
		})
	}

	// act
	start <- struct{}{}
	err := eg.Wait()

	// assert
	assert.Equal(t, nil, err)
}

func TestMemStorage_PutConcurrentDifferentKeys(t *testing.T) {
	// arrange
	storage := NewMemStorage()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	start := make(chan struct{})
	go func() {
		<-start
		wg.Done()
	}()

	eg := &errgroup.Group{}
	for i := 0; i < concurrency; i++ {
		i := i
		eg.Go(func() error {
			wg.Wait()
			return storage.Put(schema.NewCounter(fmt.Sprintf("counter_%d", i), int64(i)))
		})
		eg.Go(func() error {
			wg.Wait()
			return storage.Put(schema.NewGauge(fmt.Sprintf("gauge_%d", i), float64(i)))
		})
	}

	// act
	start <- struct{}{}
	err := eg.Wait()

	// assert
	assert.Equal(t, nil, err)
}

func TestMemStorage_ExtractFromEmpty(t *testing.T) {
	storage := NewMemStorage()
	req := schema.NewCounterRequest("counter")
	_, err := storage.Extract(req)
	assert.Errorf(t, err, "Did not return error if metrics not found")
	assert.IsType(t, &NotFound{}, err)
}

func TestMemStorage_ExtractAfterPut(t *testing.T) {
	storage := NewMemStorage()
	gauge := schema.NewGauge("gauge", 13.37)
	req := schema.NewGaugeRequest("gauge")

	_ = storage.Put(gauge)
	value, err := storage.Extract(req)

	assert.Equal(t, nil, err)
	assert.Equal(t, value, gauge)
}

func TestMemStorage_ExtractTypeMismatch(t *testing.T) {
	storage := NewMemStorage()
	gauge := schema.NewGauge("gauge", 13.37)
	req := schema.NewCounterRequest("gauge")

	_ = storage.Put(gauge)
	_, err := storage.Extract(req)

	assert.Errorf(t, err, "Did not return error on stored and requested metrics type mismatch")
	assert.IsType(t, &TypeMismatch{}, err)
}

func TestMemStorage_Increment(t *testing.T) {
	storage := NewMemStorage()
	counter := schema.NewCounter("counter", 30)
	req := schema.NewCounterRequest("counter")
	expected := schema.NewCounter("counter", 42)

	_ = storage.Put(counter)
	err := storage.Increment(req, 12)
	actual, _ := storage.Extract(req)

	assert.Equal(t, nil, err)
	assert.Equal(t, expected, actual)
}

func TestMemStorage_IncrementGauge(t *testing.T) {
	storage := NewMemStorage()
	req := schema.NewGaugeRequest("gauge")

	err := storage.Increment(req, 42)

	assert.Errorf(t, err, "Did not return error on attempt to increment gauge")
	assert.IsType(t, &IncrementingNonCounterMetrics{}, err)
}

func TestMemStorage_IncrementEmpty(t *testing.T) {
	storage := NewMemStorage()
	req := schema.NewCounterRequest("counter")

	err := storage.Increment(req, 42)

	assert.Errorf(t, err, "Did not return error on attemnt to increment non-existing value")
}

func TestMemStorage_IncrementTypeMismatch(t *testing.T) {
	storage := NewMemStorage()
	gauge := schema.NewGauge("counter", 13.37)
	req := schema.NewCounterRequest("counter")

	_ = storage.Put(gauge)
	err := storage.Increment(req, 42)

	assert.Errorf(t, err, "Did not return error on trying to increment stored gauge")
	assert.IsType(t, &TypeMismatch{}, err)
}

func TestMemStorage_IncrementConcurrentSameKey(t *testing.T) {
	// arrange
	storage := NewMemStorage()
	counter := schema.NewCounter("counter", 0)
	req := schema.NewCounterRequest("counter")
	_ = storage.Put(counter)

	wg := &sync.WaitGroup{}
	start := make(chan struct{})
	wg.Add(1)
	go func() {
		<-start
		wg.Done()
	}()

	eg := &errgroup.Group{}
	for i := 0; i < concurrency; i++ {
		eg.Go(func() error {
			wg.Wait()
			return storage.Increment(req, 42)
		})
	}

	// act
	start <- struct{}{}
	err := eg.Wait()

	// assert
	assert.Equal(t, nil, err)
}

func TestMemStorage_IncrementConcurrentDifferentKeys(t *testing.T) {
	// arrange
	storage := NewMemStorage()

	start := make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		<-start
		for i := 0; i < concurrency; i++ {
			counter := schema.NewCounter(fmt.Sprintf("counter_%d", i), 42)
			_ = storage.Put(counter)
		}
		wg.Done()
	}()

	eg := &errgroup.Group{}
	for i := 0; i < concurrency; i++ {
		// concurrent context
		i := i
		eg.Go(func() error {
			wg.Wait()
			req := schema.NewCounterRequest(fmt.Sprintf("counter_%d", i))
			return storage.Increment(req, 42)
		})
	}

	// act
	start <- struct{}{}
	err := eg.Wait()

	// assert
	assert.Equal(t, nil, err)
}

func TestMemStorage_ListEmpty(t *testing.T) {
	storage := NewMemStorage()
	actual, _ := storage.List()
	assert.Empty(t, actual)
}

func TestMemStorage_ListTrivial(t *testing.T) {
	storage := NewMemStorage()
	counter := schema.NewCounter("counter", 42)
	gauge := schema.NewGauge("gauge", 13.37)
	expected := []schema.Metrics{counter, gauge}

	_ = storage.Put(counter)
	_ = storage.Put(gauge)
	actual, _ := storage.List()

	assert.Equal(t, expected, actual)
}

// test increment actual value not requested
