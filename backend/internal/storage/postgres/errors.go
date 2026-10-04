package postgres

import (
	"errors"
	"fmt"
)

// ErrNotFound means the requested record is missing or inaccessible to its owner.
var ErrNotFound = errors.New("not found")

// ErrConflict means the conversation version changed or research is already active.
var ErrConflict = errors.New("conversation changed or already has active research")

// ErrCapacity means standard admission capacity or a daily allowance is exhausted.
var ErrCapacity = errors.New("research capacity or daily quota reached")

// ErrGroundedCapacity means the separate Google-grounded daily allowance is exhausted.
var ErrGroundedCapacity = errors.New("google-grounded daily quota reached")

// Preserve the sentinel chain so handlers can still map ownership and conflict errors.
func persistenceError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("postgres %s: %w", operation, err)
}
