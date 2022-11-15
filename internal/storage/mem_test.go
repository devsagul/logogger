package storage

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"

	"logogger/internal/schema"
)

const concurrency = 50

func TestMemStorage_PutSingle(t *testing.T) {
	storage := NewMemStorage()
	err := storage.Put(context.Background(), schema.NewCounter("counter", 42))
	assert.Equal(t, err, nil)
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
			return storage.Put(context.Background(), schema.NewCounter("counter", int64(i)))
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
			return storage.Put(context.Background(), schema.NewCounter(fmt.Sprintf("counter_%d", i), int64(i)))
		})
		eg.Go(func() error {
			wg.Wait()
			return storage.Put(context.Background(), schema.NewGauge(fmt.Sprintf("gauge_%d", i), float64(i)))
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
	_, err := storage.Extract(context.Background(), req)
	assert.Errorf(t, err, "Did not return error if metrics not found")
	assert.IsType(t, &NotFound{}, err)
}

func TestMemStorage_ExtractAfterPut(t *testing.T) {
	storage := NewMemStorage()
	gauge := schema.NewGauge("gauge", 13.37)
	req := schema.NewGaugeRequest("gauge")

	err := storage.Put(context.Background(), gauge)
	assert.NoError(t, err)
	value, err := storage.Extract(context.Background(), req)
	assert.NoError(t, err)

	assert.Equal(t, nil, err)
	assert.Equal(t, value, gauge)
}

func TestMemStorage_ExtractTypeMismatch(t *testing.T) {
	storage := NewMemStorage()
	gauge := schema.NewGauge("gauge", 13.37)
	req := schema.NewCounterRequest("gauge")

	err := storage.Put(context.Background(), gauge)
	assert.NoError(t, err)
	_, err = storage.Extract(context.Background(), req)

	assert.Errorf(t, err, "Did not return error on stored and requested metrics type mismatch")
	assert.IsType(t, &TypeMismatch{}, err)
}

func TestMemStorage_ExtractRequestedValueIgnored(t *testing.T) {
	storage := NewMemStorage()
	err := storage.Put(context.Background(), schema.NewCounter("counter", 42))
	assert.NoError(t, err)

	actual, err := storage.Extract(context.Background(), schema.NewCounter("counter", 0))
	assert.NoError(t, err)

	assert.Equal(t, int64(42), *actual.Delta)
}

func TestMemStorage_Increment(t *testing.T) {
	storage := NewMemStorage()
	counter := schema.NewCounter("counter", 30)
	req := schema.NewCounterRequest("counter")
	expected := schema.NewCounter("counter", 42)

	err := storage.Put(context.Background(), counter)
	assert.NoError(t, err)
	err = storage.Increment(context.Background(), req, 12)
	assert.NoError(t, err)
	actual, err := storage.Extract(context.Background(), req)
	assert.NoError(t, err)

	assert.Equal(t, nil, err)
	assert.Equal(t, expected, actual)
}

func TestMemStorage_IncrementGauge(t *testing.T) {
	storage := NewMemStorage()
	req := schema.NewGaugeRequest("gauge")

	err := storage.Increment(context.Background(), req, 42)

	assert.Errorf(t, err, "Did not return error on attempt to increment gauge")
	assert.IsType(t, &IncrementingNonCounterMetrics{}, err)
}

func TestMemStorage_IncrementEmpty(t *testing.T) {
	storage := NewMemStorage()
	req := schema.NewCounterRequest("counter")

	err := storage.Increment(context.Background(), req, 42)

	assert.Errorf(t, err, "Did not return error on attemnt to increment non-existing value")
}

func TestMemStorage_IncrementConcurrentSameKey(t *testing.T) {
	// arrange
	storage := NewMemStorage()
	counter := schema.NewCounter("counter", 0)
	req := schema.NewCounterRequest("counter")
	err := storage.Put(context.Background(), counter)
	assert.NoError(t, err)

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
			return storage.Increment(context.Background(), req, 42)
		})
	}

	// act
	start <- struct{}{}
	err = eg.Wait()

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
			err := storage.Put(context.Background(), counter)
			assert.NoError(t, err)
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
			return storage.Increment(context.Background(), req, 42)
		})
	}

	// act
	start <- struct{}{}
	err := eg.Wait()

	// assert
	assert.Equal(t, nil, err)
}

func TestMemStorage_IncrementRequestedValueIgnored(t *testing.T) {
	storage := NewMemStorage()
	err := storage.Put(context.Background(), schema.NewCounter("counter", 42))
	assert.NoError(t, err)
	err = storage.Increment(context.Background(), schema.NewCounter("counter", 0), 1)
	assert.NoError(t, err)

	actual, err := storage.Extract(context.Background(), schema.NewCounterRequest("counter"))
	assert.NoError(t, err)

	assert.Equal(t, int64(43), *actual.Delta)
}

func TestMemStorage_ListEmpty(t *testing.T) {
	storage := NewMemStorage()
	actual, err := storage.List(context.Background())
	assert.NoError(t, err)

	assert.Empty(t, actual)
}

func TestMemStorage_ListTrivial(t *testing.T) {
	storage := NewMemStorage()
	counter := schema.NewCounter("counter", 42)
	gauge := schema.NewGauge("gauge", 13.37)
	expected := []schema.Metrics{counter, gauge}

	err := storage.Put(context.Background(), counter)
	assert.NoError(t, err)
	err = storage.Put(context.Background(), gauge)
	assert.NoError(t, err)
	actual, err := storage.List(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)
}
