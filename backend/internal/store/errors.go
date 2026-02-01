package store

import "errors"

var (
	ErrConflict            = errors.New("conflict")
	ErrNotFound            = errors.New("not found")
	ErrIdempotencyConflict = errors.New("idempotency key conflict")
)
