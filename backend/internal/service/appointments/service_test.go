package appointments

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"schedula/backend/internal/domain"
	"schedula/backend/internal/store"
)

type fakeRepo struct {
	createFn              func(ctx context.Context, appt domain.Appointment) (domain.Appointment, error)
	listFn                func(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error)
	deleteFn              func(ctx context.Context, userID string, appointmentID uuid.UUID) error
	createRecurringSeries func(ctx context.Context, series domain.RecurringSeries) (domain.RecurringSeries, error)
	listOccurrences       func(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.RecurringOccurrence, error)
}

func (f *fakeRepo) Create(ctx context.Context, appt domain.Appointment) (domain.Appointment, error) {
	if f.createFn == nil {
		panic("Create not configured")
	}
	return f.createFn(ctx, appt)
}

func (f *fakeRepo) List(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error) {
	if f.listFn == nil {
		panic("List not configured")
	}
	return f.listFn(ctx, userID, windowStart, windowEnd)
}

func (f *fakeRepo) Delete(ctx context.Context, userID string, appointmentID uuid.UUID) error {
	if f.deleteFn == nil {
		panic("Delete not configured")
	}
	return f.deleteFn(ctx, userID, appointmentID)
}

func (f *fakeRepo) CreateRecurringSeries(ctx context.Context, series domain.RecurringSeries) (domain.RecurringSeries, error) {
	if f.createRecurringSeries == nil {
		panic("CreateRecurringSeries not configured")
	}
	return f.createRecurringSeries(ctx, series)
}

func (f *fakeRepo) ListOccurrences(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.RecurringOccurrence, error) {
	if f.listOccurrences == nil {
		panic("ListOccurrences not configured")
	}
	return f.listOccurrences(ctx, userID, windowStart, windowEnd)
}

func TestServiceCreate_ValidationErrorType(t *testing.T) {
	svc := NewService(&fakeRepo{
		createFn: func(ctx context.Context, appt domain.Appointment) (domain.Appointment, error) {
			return appt, nil
		},
	})

	_, err := svc.Create(context.Background(), CreateInput{
		UserID:    "",
		Title:     "x",
		StartTime: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
	if vErr.Error() != "user_id is required" {
		t.Fatalf("error = %q, want %q", vErr.Error(), "user_id is required")
	}
}

func TestServiceCreate_TrimsTitleAndNormalizesTimesToUTC(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("LoadLocation error: %v", err)
	}

	var got domain.Appointment
	svc := NewService(&fakeRepo{
		createFn: func(ctx context.Context, appt domain.Appointment) (domain.Appointment, error) {
			got = appt
			return appt, nil
		},
	})

	startLocal := time.Date(2026, 1, 10, 9, 0, 0, 0, loc)
	endLocal := time.Date(2026, 1, 10, 10, 0, 0, 0, loc)

	_, err = svc.Create(context.Background(), CreateInput{
		UserID:    "u1",
		Title:     "  hello  ",
		StartTime: startLocal,
		EndTime:   endLocal,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if got.Title != "hello" {
		t.Fatalf("title = %q, want %q", got.Title, "hello")
	}
	if got.StartTime.Location() != time.UTC || got.EndTime.Location() != time.UTC {
		t.Fatalf("expected UTC times, got start=%v end=%v", got.StartTime, got.EndTime)
	}
	if !got.EndTime.After(got.StartTime) {
		t.Fatalf("end_time must be after start_time")
	}
}

func TestServiceCreate_IdempotencyKeyDeterministicUUID(t *testing.T) {
	var ids []uuid.UUID
	svc := NewService(&fakeRepo{
		createFn: func(ctx context.Context, appt domain.Appointment) (domain.Appointment, error) {
			ids = append(ids, appt.ID)
			return appt, nil
		},
	})

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	_, err := svc.Create(context.Background(), CreateInput{
		UserID:         "u1",
		Title:          "t",
		StartTime:      start,
		EndTime:        end,
		IdempotencyKey: "k1",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	_, err = svc.Create(context.Background(), CreateInput{
		UserID:         "u1",
		Title:          "t",
		StartTime:      start,
		EndTime:        end,
		IdempotencyKey: "k1",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("captured ids = %d, want 2", len(ids))
	}
	if ids[0] == uuid.Nil || ids[1] == uuid.Nil {
		t.Fatalf("expected non-nil ids")
	}
	if ids[0] != ids[1] {
		t.Fatalf("ids differ: %s vs %s", ids[0], ids[1])
	}
}

func TestServiceCreate_IdempotencyKeyDifferentKeyDifferentUUID(t *testing.T) {
	var ids []uuid.UUID
	svc := NewService(&fakeRepo{
		createFn: func(ctx context.Context, appt domain.Appointment) (domain.Appointment, error) {
			ids = append(ids, appt.ID)
			return appt, nil
		},
	})

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	_, err := svc.Create(context.Background(), CreateInput{
		UserID:         "u1",
		Title:          "t",
		StartTime:      start,
		EndTime:        end,
		IdempotencyKey: "k1",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	_, err = svc.Create(context.Background(), CreateInput{
		UserID:         "u1",
		Title:          "t",
		StartTime:      start,
		EndTime:        end,
		IdempotencyKey: "k2",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("captured ids = %d, want 2", len(ids))
	}
	if ids[0] == ids[1] {
		t.Fatalf("expected different ids, got %s", ids[0])
	}
}

func TestServiceCreate_PropagatesStoreErrors(t *testing.T) {
	svc := NewService(&fakeRepo{
		createFn: func(ctx context.Context, appt domain.Appointment) (domain.Appointment, error) {
			return domain.Appointment{}, store.ErrConflict
		},
	})

	_, err := svc.Create(context.Background(), CreateInput{
		UserID:    "u1",
		Title:     "t",
		StartTime: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("error = %v, want %v", err, store.ErrConflict)
	}
}

func TestServiceCreateRecurringSeries_DefaultWeekdayAndIntervalNormalization(t *testing.T) {
	count := 1
	var got domain.RecurringSeries

	svc := NewService(&fakeRepo{
		createRecurringSeries: func(ctx context.Context, series domain.RecurringSeries) (domain.RecurringSeries, error) {
			got = series
			return series, nil
		},
	})

	start := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	_, err := svc.CreateRecurringSeries(context.Background(), CreateRecurringSeriesInput{
		UserID:    "u1",
		Title:     "t",
		StartTime: start,
		EndTime:   end,
		Rule: RecurrenceRuleInput{
			Interval: 0,
			Count:    &count,
			TimeZone: "UTC",
		},
	})
	if err != nil {
		t.Fatalf("CreateRecurringSeries error: %v", err)
	}

	if got.Interval != 1 {
		t.Fatalf("interval = %d, want 1", got.Interval)
	}
	if len(got.ByWeekday) != 1 || got.ByWeekday[0] != 1 {
		t.Fatalf("byweekday = %v, want [1]", got.ByWeekday)
	}
}

func TestServiceCreateRecurringSeries_RequiresUntilOrCount(t *testing.T) {
	svc := NewService(&fakeRepo{
		createRecurringSeries: func(ctx context.Context, series domain.RecurringSeries) (domain.RecurringSeries, error) {
			return series, nil
		},
	})

	start := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	_, err := svc.CreateRecurringSeries(context.Background(), CreateRecurringSeriesInput{
		UserID:    "u1",
		Title:     "t",
		StartTime: start,
		EndTime:   end,
		Rule: RecurrenceRuleInput{
			TimeZone: "UTC",
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
	if vErr.Error() != "until or count is required" {
		t.Fatalf("error = %q, want %q", vErr.Error(), "until or count is required")
	}
}

