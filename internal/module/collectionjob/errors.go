package collectionjob

import (
	"errors"
	"fmt"

	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
)

type permanentError struct {
	err error
}

func (e permanentError) Error() string {
	return e.err.Error()
}

func (e permanentError) Unwrap() error {
	return e.err
}

func PermanentError(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

func IsPermanentError(err error) bool {
	var permanent permanentError
	return errors.As(err, &permanent) ||
		errors.Is(err, source.ErrSourceNotAccessible) ||
		errors.Is(err, collector.ErrCollectorNotFound)
}

func RetryableError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("retryable collection job error: %w", err)
}
