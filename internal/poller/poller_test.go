package poller

import (
	"logogger/internal/schema"
	"testing"

	"github.com/stretchr/testify/assert"
)

func extractCounter(t *testing.T, p Poller) int64 {
	_, err := p.Poll()
	if err != nil {
		assert.FailNow(t, "Error accessing storage.")
	}
	value, err := p.store.Extract(schema.NewCounterRequest("PollCount"))
	if err != nil {
		assert.FailNow(t, "Error accessing storage.")
	}
	return *value.Delta
}

func TestPoller(t *testing.T) {
	p, err := NewPoller(0)
	if err != nil {
		assert.FailNow(t, "Error accessing storage.")
	}

	c1 := extractCounter(t, p)
	c2 := extractCounter(t, p)
	err = p.Reset()
	if err != nil {
		assert.FailNow(t, "Error resetting poller.")
	}
	c3 := extractCounter(t, p)

	assert.Equal(t, int64(1), c1)
	assert.Equal(t, int64(2), c2)
	assert.Equal(t, int64(1), c3)
}
