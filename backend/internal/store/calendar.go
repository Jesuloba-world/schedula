package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"schedula/backend/internal/domain"
)

type CalendarTx interface {
	CreateAppointment(ctx context.Context, appt domain.Appointment) (domain.Appointment, error)
	ListAppointments(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error)
	DeleteAppointment(ctx context.Context, userID string, appointmentID uuid.UUID) error

	CreateRecurringSeries(ctx context.Context, series domain.RecurringSeries) (domain.RecurringSeries, error)
	ListRecurringSeries(ctx context.Context, userID string) ([]domain.RecurringSeries, error)
	ListRecurringExceptions(ctx context.Context, seriesID uuid.UUID, windowStart, windowEnd time.Time) ([]domain.RecurringException, error)
	UpsertRecurringException(ctx context.Context, ex domain.RecurringException) (domain.RecurringException, error)
	DeleteRecurringSeries(ctx context.Context, userID string, seriesID uuid.UUID) error
}
