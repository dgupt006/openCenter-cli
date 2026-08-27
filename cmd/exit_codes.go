package cmd

import (
	"errors"
	"fmt"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// ExitError carries a user-facing error category and its process exit code.
type ExitError struct {
	Code    int
	Message string
	Err     error
}

func NewExitError(code int, message string, cause error) *ExitError {
	return &ExitError{Code: code, Message: message, Err: cause}
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *ExitError) Unwrap() error { return e.Err }

// ExitCode maps command errors to the CLI process contract.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var configNotFound *v2.ConfigNotFoundError
	if errors.As(err, &configNotFound) {
		return 3
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) && exitErr.Code >= 0 {
		return exitErr.Code
	}
	if v2.IsValidationError(err) {
		return 2
	}
	return 1
}
