package storage

import (
	"logogger/internal/schema"
	"sort"
	"sync"
)

type MemStorage struct {
	m map[string]schema.Metrics

	/**
	Here lies the bright idea of using mutexes
	for every key separately. Unfortunately,
	concurrent read-write operations into map
	are not available (although, it would suit me
	to lock concurrent write/increment per key only).

	So I use only one mutex for the whole storage.
	*/
	mu sync.Mutex
}

func (storage *MemStorage) Put(req schema.Metrics) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	cur, found := storage.m[req.ID]
	if found && cur.MType != req.MType {
		return typeMismatch(req.ID, req.MType, cur.MType)
	}

	storage.m[req.ID] = req
	return nil
}

func (storage *MemStorage) Extract(req schema.Metrics) (schema.Metrics, error) {
	// Note: this extract should not be re-used in other
	// methods, this would require a recursive mutex
	storage.mu.Lock()
	value, found := storage.m[req.ID]
	storage.mu.Unlock()
	if !found {
		return req, notFound(req.ID)
	}
	if req.MType != value.MType {
		return req, typeMismatch(req.ID, req.MType, value.MType)
	}
	return value, nil
}

func (storage *MemStorage) Increment(req schema.Metrics, value int64) error {
	if req.MType != "counter" {
		return incrementingNonCounterMetrics(req.ID, req.MType)
	}

	storage.mu.Lock()
	defer storage.mu.Unlock()

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

func (storage *MemStorage) List() ([]schema.Metrics, error) {
	var res []schema.Metrics

	for _, value := range storage.m {
		res = append(res, value)
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].ID < res[j].ID
	})

	return res, nil
}

func (storage *MemStorage) BulkPut(values []schema.Metrics) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	for _, req := range values {
		cur, found := storage.m[req.ID]
		if found && cur.MType != req.MType {
			return typeMismatch(req.ID, req.MType, cur.MType)
		}

		storage.m[req.ID] = req

	}
	return nil
}

func NewMemStorage() *MemStorage {
	m := new(MemStorage)
	m.m = map[string]schema.Metrics{}
	return m
}
