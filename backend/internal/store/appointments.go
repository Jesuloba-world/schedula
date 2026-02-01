package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"schedula/backend/internal/domain"
)

const RecurringConflictLookahead = 180 * 24 * time.Hour

type AppointmentRepository interface {
	Create(ctx context.Context, appt domain.Appointment) (domain.Appointment, error)
	List(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error)
	Delete(ctx context.Context, userID string, appointmentID uuid.UUID) error

	CreateRecurringSeries(ctx context.Context, series domain.RecurringSeries) (domain.RecurringSeries, error)
	ListOccurrences(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.RecurringOccurrence, error)
}
