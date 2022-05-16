package poller

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPoller(t *testing.T) {
	poller, reset := Poller(0)

	m1 := poller()
	m2 := poller()
	reset()
	m3 := poller()
	m4 := poller()
	m5 := poller()

	assert.NotEqual(t, nil, m1)
	assert.Equal(t, counter(1), m1.PollCount)
	assert.Equal(t, counter(2), m2.PollCount)
	assert.Equal(t, counter(1), m3.PollCount)
	assert.Equal(t, counter(2), m4.PollCount)
	assert.Equal(t, counter(3), m5.PollCount)
}
