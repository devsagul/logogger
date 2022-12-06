package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"

	"logogger/internal/storage"
)

type applicationError struct {
	wrapped    error
	httpStatus int
	grpcStatus codes.Code
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

func convertError(e error) *applicationError {
	if e == nil {
		return nil
	}
	switch err := e.(type) {
	case nil:
		return nil
	case *validationError:
		return &applicationError{
			err,
			http.StatusBadRequest,
			codes.InvalidArgument,
		}
	case *storage.NotFound:
		return &applicationError{
			fmt.Errorf("could not find metrics with name %s", err.ID),
			http.StatusNotFound,
			codes.NotFound,
		}
	case *storage.IncrementingNonCounterMetrics:
		return &applicationError{
			fmt.Errorf("could not increment metrics of type %s", err.ActualType),
			http.StatusNotImplemented,
			codes.Unimplemented,
		}
	case *storage.TypeMismatch:
		return &applicationError{
			fmt.Errorf("requested operation on metrics %s with type %s, but actual type in storage is %s", err.ID, err.Requested, err.Stored),
			http.StatusConflict,
			codes.InvalidArgument, // debatable
		}
	default:
		return &applicationError{
			errors.New("internal server error"),
			http.StatusInternalServerError,
			codes.Internal,
		}
	}
}
