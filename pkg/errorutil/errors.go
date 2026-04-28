package errorutil

import (
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"
)

// Convert compares the error and maps it to a appropriate connect.Error
// if there is no backend-specific wrapped error given to this function, it will just return an internal error.
// if there are multiple errors wrapped this function only cares about the first found error in the error tree.
func Convert(err error) *connect.Error {
	// Ipam or other connect errors
	if connectErr, ok := errors.AsType[*connect.Error](err); ok {
		// when the connect error is wrapped deeper a tree, connect.Error() calls the string function on
		// the error and adds things like "internal: ..."
		// so we replace the wrapped error message with the direct message
		cleaned := strings.Replace(err.Error(), connectErr.Error(), connectErr.Message(), 1)
		return connect.NewError(connectErr.Code(), errors.New(cleaned))
	}

	return connect.NewError(connect.CodeInternal, err)
}

func IsNotFound(err error) bool {
	connectErr := Convert(err)
	return connectErr.Code() == connect.CodeNotFound
}

func IsConflict(err error) bool {
	connectErr := Convert(err)
	return connectErr.Code() == connect.CodeAlreadyExists
}

func IsInternal(err error) bool {
	connectErr := Convert(err)
	return connectErr.Code() == connect.CodeInternal
}

func IsInvalidArgument(err error) bool {
	connectErr := Convert(err)
	return connectErr.Code() == connect.CodeInvalidArgument
}

func IsFailedPrecondition(err error) bool {
	connectErr := Convert(err)
	return connectErr.Code() == connect.CodeFailedPrecondition
}

// NotFound creates a new notfound error with a given error message.
func NotFound(format string, args ...any) error {
	return connect.NewError(connect.CodeNotFound, fmt.Errorf(format, args...))
}

// Conflict creates a new conflict error with a given error message.
func Conflict(format string, args ...any) error {
	return connect.NewError(connect.CodeAlreadyExists, fmt.Errorf(format, args...))
}

// Internal creates a new Internal error with a given error message and the original error.
func Internal(format string, args ...any) error {
	return connect.NewError(connect.CodeInternal, fmt.Errorf(format, args...))
}

// InvalidArgument creates a new InvalidArgument error with a given error message and the original error.
func InvalidArgument(format string, args ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, args...))
}

// FailedPrecondition creates a new FailedPrecondition error with a given error message and the original error.
func FailedPrecondition(format string, args ...any) error {
	return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf(format, args...))
}

func ConnectErrorComparer() cmp.Option {
	return cmp.Comparer(func(x, y *connect.Error) bool {
		if x == nil && y == nil {
			return true
		}
		if x == nil && y != nil {
			return false
		}
		if x != nil && y == nil {
			return false
		}
		if x.Error() != y.Error() {
			return false
		}
		return x.Code() == y.Code()
	})
}
