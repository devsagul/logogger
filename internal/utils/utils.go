package utils

import (
	"errors"
	"log"
	"time"
)

type goroutine = func()
type errGoroutine = func() error

func WrapGoroutinePanic(g errGoroutine) errGoroutine {
	return func() (err error) {
		defer func() {
			r := recover()
			if r != nil {
				log.Printf("Panic in goroutine: %s", r)
				err = errors.New("panic during goroutine execution")
			}
		}()
		err = g()
		return err
	}
}

func RetryForever(g errGoroutine, t time.Duration) goroutine {
	return func() {
		for err := g(); err != nil; {
			log.Printf("Error during function envocation: %s, will be retried in %s", err.Error(), t)
			<-time.NewTimer(t).C
			err = g()
		}
	}
}
