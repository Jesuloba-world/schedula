package grpc

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"schedula/backend/internal/domain"
	schedulev1 "schedula/backend/internal/gen/proto/schedula/v1"
	"schedula/backend/internal/service/appointments"
	"schedula/backend/internal/store"
)

type fakeAppointmentsService struct {
	createFn              func(ctx context.Context, in appointments.CreateInput) (domain.Appointment, error)
	listFn                func(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error)
	deleteFn              func(ctx context.Context, userID string, appointmentID uuid.UUID) error
	createRecurringSeries func(ctx context.Context, in appointments.CreateRecurringSeriesInput) (domain.RecurringSeries, error)
	listOccurrencesFn     func(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.RecurringOccurrence, error)
}

func (f *fakeAppointmentsService) Create(ctx context.Context, in appointments.CreateInput) (domain.Appointment, error) {
	if f.createFn == nil {
		panic("Create not configured")
	}
	return f.createFn(ctx, in)
}

func (f *fakeAppointmentsService) List(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error) {
	if f.listFn == nil {
		panic("List not configured")
	}
	return f.listFn(ctx, userID, windowStart, windowEnd)
}

func (f *fakeAppointmentsService) Delete(ctx context.Context, userID string, appointmentID uuid.UUID) error {
	if f.deleteFn == nil {
		panic("Delete not configured")
	}
	return f.deleteFn(ctx, userID, appointmentID)
}

func (f *fakeAppointmentsService) CreateRecurringSeries(ctx context.Context, in appointments.CreateRecurringSeriesInput) (domain.RecurringSeries, error) {
	if f.createRecurringSeries == nil {
		panic("CreateRecurringSeries not configured")
	}
	return f.createRecurringSeries(ctx, in)
}

func (f *fakeAppointmentsService) ListOccurrences(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.RecurringOccurrence, error) {
	if f.listOccurrencesFn == nil {
		panic("ListOccurrences not configured")
	}
	return f.listOccurrencesFn(ctx, userID, windowStart, windowEnd)
}

func TestIdempotencyKey_ReadsHeadersAndTrims(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("idempotency-key", "  abc  "))
	if got := idempotencyKey(ctx); got != "abc" {
		t.Fatalf("idempotencyKey = %q, want %q", got, "abc")
	}

	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-idempotency-key", "xyz"))
	if got := idempotencyKey(ctx); got != "xyz" {
		t.Fatalf("idempotencyKey = %q, want %q", got, "xyz")
	}
}

func TestCreateAppointment_RejectsMissingTimes(t *testing.T) {
	srv := NewAppointmentsServer(&fakeAppointmentsService{
		createFn: func(ctx context.Context, in appointments.CreateInput) (domain.Appointment, error) {
			return domain.Appointment{}, nil
		},
	}, slog.Default())

	_, err := srv.CreateAppointment(context.Background(), &schedulev1.CreateAppointmentRequest{
		UserId: "u1",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestCreateAppointment_PassesIdempotencyKeyToService(t *testing.T) {
	var gotKey string

	srv := NewAppointmentsServer(&fakeAppointmentsService{
		createFn: func(ctx context.Context, in appointments.CreateInput) (domain.Appointment, error) {
			gotKey = in.IdempotencyKey
			return domain.Appointment{ID: uuid.MustParse("00000000-0000-0000-0000-000000000010")}, nil
		},
	}, slog.Default())

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("idempotency-key", "k1"))

	_, err := srv.CreateAppointment(ctx, &schedulev1.CreateAppointmentRequest{
		UserId:    "u1",
		Title:     "t",
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	})
	if err != nil {
		t.Fatalf("CreateAppointment error: %v", err)
	}
	if gotKey != "k1" {
		t.Fatalf("idempotency_key = %q, want %q", gotKey, "k1")
	}
}

func TestCreateAppointment_MapsConflict(t *testing.T) {
	srv := NewAppointmentsServer(&fakeAppointmentsService{
		createFn: func(ctx context.Context, in appointments.CreateInput) (domain.Appointment, error) {
			return domain.Appointment{}, store.ErrConflict
		},
	}, slog.Default())

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	_, err := srv.CreateAppointment(context.Background(), &schedulev1.CreateAppointmentRequest{
		UserId:    "u1",
		Title:     "t",
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}

func TestCreateAppointment_MapsValidationError(t *testing.T) {
	srv := NewAppointmentsServer(&fakeAppointmentsService{
		createFn: func(ctx context.Context, in appointments.CreateInput) (domain.Appointment, error) {
			return domain.Appointment{}, &appointments.ValidationError{}
		},
	}, slog.Default())

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	_, err := srv.CreateAppointment(context.Background(), &schedulev1.CreateAppointmentRequest{
		UserId:    "u1",
		Title:     "t",
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestDeleteAppointment_RejectsInvalidUUID(t *testing.T) {
	srv := NewAppointmentsServer(&fakeAppointmentsService{
		deleteFn: func(ctx context.Context, userID string, appointmentID uuid.UUID) error {
			return nil
		},
	}, slog.Default())

	_, err := srv.DeleteAppointment(context.Background(), &schedulev1.DeleteAppointmentRequest{
		UserId:        "u1",
		AppointmentId: "not-a-uuid",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}

func TestDeleteAppointment_MapsNotFound(t *testing.T) {
	srv := NewAppointmentsServer(&fakeAppointmentsService{
		deleteFn: func(ctx context.Context, userID string, appointmentID uuid.UUID) error {
			return store.ErrNotFound
		},
	}, slog.Default())

	_, err := srv.DeleteAppointment(context.Background(), &schedulev1.DeleteAppointmentRequest{
		UserId:        "u1",
		AppointmentId: "00000000-0000-0000-0000-000000000020",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %s, want %s", status.Code(err), codes.NotFound)
	}
}

func TestCreateAppointment_MapsIdempotencyConflict(t *testing.T) {
	srv := NewAppointmentsServer(&fakeAppointmentsService{
		createFn: func(ctx context.Context, in appointments.CreateInput) (domain.Appointment, error) {
			return domain.Appointment{}, store.ErrIdempotencyConflict
		},
	}, slog.Default())

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	_, err := srv.CreateAppointment(context.Background(), &schedulev1.CreateAppointmentRequest{
		UserId:    "u1",
		Title:     "t",
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %s, want %s", status.Code(err), codes.FailedPrecondition)
	}
}
