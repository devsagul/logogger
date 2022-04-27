package storage

import (
	"fmt"
	"sort"
	"sync"
)

type MemStorage struct {
	m  map[string]MetricDef
	mu sync.Mutex
}

func (storage *MemStorage) Increment(key string, value interface{}) error {
	p, ok := value.(int64)
	if !ok {
		return fmt.Errorf("could not increment value %s, increment value %s is not int64", key, value)
	}

	storage.mu.Lock()
	defer storage.mu.Unlock()
	prev, found := storage.m[key]
	if found && prev.Type != "counter" {
		return fmt.Errorf("could not increment value %s, currently it's holding value of type %s", key, prev.Type)
	}
	if !found {
		prev = MetricDef{
			"coutner",
			key,
			int64(0),
		}
	}

	v := prev.Value.(int64)
	storage.m[key] = MetricDef{
		"counter",
		key,
		v + p,
	}
	return nil
}

func (storage *MemStorage) Get(key string) (MetricDef, bool, error) {
	value, found := storage.m[key]
	if !found {
		return MetricDef{}, false, nil
	} else {
		return value, true, nil
	}
}

func (storage *MemStorage) Put(key string, value MetricDef) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.m[key] = value
	return nil
}

func (storage *MemStorage) List() ([]MetricDef, error) {
	var res []MetricDef

	for _, value := range storage.m {
		res = append(res, value)
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})

	return res, nil
}

func NewMemStorage() *MemStorage {
	m := new(MemStorage)
	m.m = map[string]MetricDef{}
	return m
}
