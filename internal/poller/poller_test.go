package poller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"logogger/internal/schema"
)

func extractCounter(t *testing.T, p Poller) int64 {
	_, err := p.Poll(context.Background())
	if err != nil {
		assert.FailNow(t, "Error accessing storage.")
	}
	value, err := p.store.Extract(context.Background(), schema.NewCounterRequest("PollCount"))
	if err != nil {
		assert.FailNow(t, "Error accessing storage.")
	}
	return *value.Delta
}

func TestPoller(t *testing.T) {
	p, err := NewPoller(context.Background(), 0)
	if err != nil {
		assert.FailNow(t, "Error accessing storage.")
	}

	c1 := extractCounter(t, p)
	c2 := extractCounter(t, p)
	err = p.Reset(context.Background())
	if err != nil {
		assert.FailNow(t, "Error resetting poller.")
	}
	c3 := extractCounter(t, p)

	assert.Equal(t, int64(1), c1)
	assert.Equal(t, int64(2), c2)
	assert.Equal(t, int64(1), c3)
}
