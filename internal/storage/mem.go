package storage

import (
	"context"
	"sort"
	"sync"

	"logogger/internal/schema"
)

type MemStorage struct {
	/**
	Here lies the bright idea of using mutexes
	for every key separately. Unfortunately,
	concurrent read-write operations into map
	are not available (although, it would suit me
	to lock concurrent write/increment per key only).

	So I use only one mutex for the whole storage.
	*/
	m map[string]schema.Metrics
	sync.Mutex
}

func (storage *MemStorage) Put(_ context.Context, req schema.Metrics) error {
	storage.Lock()
	defer storage.Unlock()
	storage.m[req.ID] = req
	return nil
}

func (storage *MemStorage) Extract(_ context.Context, req schema.Metrics) (schema.Metrics, error) {
	// Note: this extract should not be re-used in other
	// methods, this would require a recursive mutex
	storage.Lock()
	value, found := storage.m[req.ID]
	storage.Unlock()
	if !found {
		return req, notFound(req.ID)
	}
	if req.MType != value.MType {
		return req, typeMismatch(req.ID, req.MType, value.MType)
	}
	return value, nil
}

func (storage *MemStorage) Increment(_ context.Context, req schema.Metrics, value int64) error {
	if req.MType != schema.MetricsTypeCounter {
		return incrementingNonCounterMetrics(req.ID, req.MType)
	}

	storage.Lock()
	defer storage.Unlock()

	current, found := storage.m[req.ID]

	if !found {
		// Note: I do not assume any required behaviour here,
		// it's up to application to decide, whether this is an
		// error or the value should be just set as is.
		return notFound(req.ID)
	}

	if req.MType != current.MType {
		// Note: I do not assume any required behaviour here,
		// it's up to application to decide, whether this is an
		// error or the value in storage should change its type and
		// be reset.
		return typeMismatch(req.ID, req.MType, current.MType)
	}

	delta := *current.Delta + value
	req.Delta = &delta
	storage.m[req.ID] = req
	return nil
}

func (storage *MemStorage) List(_ context.Context) ([]schema.Metrics, error) {
	var res []schema.Metrics

	for _, value := range storage.m {
		res = append(res, value)
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].ID < res[j].ID
	})

	return res, nil
}

func (storage *MemStorage) BulkPut(_ context.Context, values []schema.Metrics) error {
	storage.Lock()
	defer storage.Unlock()
	for _, req := range values {
		storage.m[req.ID] = req
	}
	return nil
}

func (storage *MemStorage) BulkUpdate(_ context.Context, counters []schema.Metrics, gauges []schema.Metrics) error {
	storage.Lock()
	defer storage.Unlock()
	for _, counter := range counters {
		prev, found := storage.m[counter.ID]
		if found {
			value := *prev.Delta + *counter.Delta
			counter.Delta = &value
		}
		storage.m[counter.ID] = counter
	}
	for _, gauge := range gauges {
		storage.m[gauge.ID] = gauge
	}
	return nil
}

func (*MemStorage) Ping(_ context.Context) error {
	return nil
}

func (*MemStorage) Close() error {
	return nil
}

func NewMemStorage() *MemStorage {
	m := new(MemStorage)
	m.m = map[string]schema.Metrics{}
	return m
}
