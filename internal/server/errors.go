package server

import "strings"

type requestError struct {
	status int
	body   string
}

func (e *requestError) Error() string {
	return e.body
}

type validationError struct {
	messages []string
}

func (e *validationError) Error() string {
	return "Validation errors: " + strings.Join(e.messages, ";")
}

func ValidationError(messages ...string) *validationError {
	return &validationError{messages}
}
