package utils

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWrapGouritnePanic(t *testing.T) {
	t.Run("Wrap goroutine without error", func(t *testing.T) {
		err := WrapGoroutinePanic(func() error { return nil })()
		assert.NoError(t, err)
	})

	t.Run("Wrap goroutine with error", func(t *testing.T) {
		err := errors.New("generic error")
		actual := WrapGoroutinePanic(func() error { return err })()
		assert.Equal(t, err, actual)
	})

	t.Run("Wrap goroutine with panic", func(t *testing.T) {
		defer func() {
			r := recover()
			if r != nil {
				t.Fail()
				t.Log("Panic haven't been recovered in goroutine")
			}
		}()
		err := WrapGoroutinePanic(func() error { panic("inner panic in the goroutine") })()
		assert.Error(t, err)
	})
}

func callCounter(t *testing.T, c *int, f errGoroutine) errGoroutine {
	if c == nil {
		t.Log("Cannot instantiate call counter with argument (nil)")
		t.FailNow()
	}
	return func() error {
		defer func() { *c++ }()
		return f()
	}
}

func TestRetryForever(t *testing.T) {
	t.Run("Retry goroutine without error", func(t *testing.T) {
		n := 0
		counter := callCounter(t, &n, func() error { return nil })
		RetryForever(counter, 1*time.Second)()
		assert.Equal(t, 1, n)
	})
	t.Run("Retry goroutine with error", func(t *testing.T) {
		n := 0
		counter := callCounter(t, &n, func() error {
			if n == 0 {
				return errors.New("generic error")
			}
			return nil
		})
		RetryForever(counter, 1*time.Microsecond)()
		assert.Equal(t, 2, n)
	})
}
