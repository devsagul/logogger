package poller

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPoller(t *testing.T) {
	poller, reset := Poller(0)

	m1 := poller()
	m2 := poller()
	reset <- struct{}{}
	m3 := poller()

	// TODO check that all items are present within the polled data
	assert.NotEqual(t, nil, m1)
	assert.Equal(t, counter(1), m1.PollCount)
	assert.Equal(t, counter(2), m2.PollCount)
	assert.Equal(t, counter(1), m3.PollCount)
}
