// Package utils implements different project-wide utils
package utils

import (
	"errors"
	"fmt"
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

func coalesceString(s string, val string) string {
	if string != "" {
		return s
	}
	return val
}

func PrintVersionInfo(buildVersion, buildDate, buildCommit string) {
	fmt.Printf("Build version: %s", coalesceString(buildVersion, "N/A"))
	fmt.Printf("Build date: %s", coalesceString(buildVersion, "N/A"))
	fmt.Printf("Build commit: %s", coalesceString(buildVersion, "N/A"))
}
