package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"schedula/backend/internal/domain"
	"schedula/backend/internal/store"
)

type fakeCalendarTx struct {
	listAppointmentsFn        func(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error)
	listRecurringSeriesFn     func(ctx context.Context, userID string) ([]domain.RecurringSeries, error)
	listRecurringExceptionsFn func(ctx context.Context, seriesID uuid.UUID, windowStart, windowEnd time.Time) ([]domain.RecurringException, error)
}

func (f *fakeCalendarTx) CreateAppointment(ctx context.Context, appt domain.Appointment) (domain.Appointment, error) {
	panic("not used")
}

func (f *fakeCalendarTx) ListAppointments(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error) {
	if f.listAppointmentsFn == nil {
		return nil, nil
	}
	return f.listAppointmentsFn(ctx, userID, windowStart, windowEnd)
}

func (f *fakeCalendarTx) DeleteAppointment(ctx context.Context, userID string, appointmentID uuid.UUID) error {
	panic("not used")
}

func (f *fakeCalendarTx) CreateRecurringSeries(ctx context.Context, series domain.RecurringSeries) (domain.RecurringSeries, error) {
	panic("not used")
}

func (f *fakeCalendarTx) ListRecurringSeries(ctx context.Context, userID string) ([]domain.RecurringSeries, error) {
	if f.listRecurringSeriesFn == nil {
		return nil, nil
	}
	return f.listRecurringSeriesFn(ctx, userID)
}

func (f *fakeCalendarTx) ListRecurringExceptions(ctx context.Context, seriesID uuid.UUID, windowStart, windowEnd time.Time) ([]domain.RecurringException, error) {
	if f.listRecurringExceptionsFn == nil {
		return nil, nil
	}
	return f.listRecurringExceptionsFn(ctx, seriesID, windowStart, windowEnd)
}

func (f *fakeCalendarTx) UpsertRecurringException(ctx context.Context, ex domain.RecurringException) (domain.RecurringException, error) {
	panic("not used")
}

func (f *fakeCalendarTx) DeleteRecurringSeries(ctx context.Context, userID string, seriesID uuid.UUID) error {
	panic("not used")
}

func TestApplyRecurringExceptions(t *testing.T) {
	baseTime := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	windowStart := baseTime
	windowEnd := baseTime.Add(24 * time.Hour)

	occs := []domain.RecurringOccurrence{
		{
			ID:        "1",
			SeriesID:  uuid.MustParse("00000000-0000-0000-0000-000000000101"),
			UserID:    "u1",
			Title:     "t1",
			Notes:     "n1",
			StartTime: baseTime,
			EndTime:   baseTime.Add(time.Hour),
		},
		{
			ID:        "2",
			SeriesID:  uuid.MustParse("00000000-0000-0000-0000-000000000101"),
			UserID:    "u1",
			Title:     "t2",
			Notes:     "n2",
			StartTime: baseTime.Add(2 * time.Hour),
			EndTime:   baseTime.Add(3 * time.Hour),
		},
	}

	t.Run("skip removes occurrence", func(t *testing.T) {
		exs := []domain.RecurringException{
			{
				SeriesID:        occs[0].SeriesID,
				OccurrenceStart: occs[0].StartTime,
				Kind:            domain.RecurringExceptionKindSkip,
			},
		}

		out := applyRecurringExceptions(occs, exs, windowStart, windowEnd)
		if len(out) != 1 {
			t.Fatalf("len(out) = %d, want 1", len(out))
		}
		if out[0].ID != "2" {
			t.Fatalf("kept occurrence id = %q, want %q", out[0].ID, "2")
		}
	})

	t.Run("override updates fields", func(t *testing.T) {
		overrideTitle := "new title"
		overrideNotes := "new notes"
		overrideStart := baseTime.Add(30 * time.Minute)
		overrideEnd := baseTime.Add(90 * time.Minute)

		exs := []domain.RecurringException{
			{
				SeriesID:        occs[0].SeriesID,
				OccurrenceStart: occs[0].StartTime,
				Kind:            domain.RecurringExceptionKindOverride,
				OverrideStart:   &overrideStart,
				OverrideEnd:     &overrideEnd,
				OverrideTitle:   &overrideTitle,
				OverrideNotes:   &overrideNotes,
			},
		}

		out := applyRecurringExceptions(occs, exs, windowStart, windowEnd)
		if len(out) != 2 {
			t.Fatalf("len(out) = %d, want 2", len(out))
		}
		if out[0].Title != overrideTitle || out[0].Notes != overrideNotes {
			t.Fatalf("override fields not applied: title=%q notes=%q", out[0].Title, out[0].Notes)
		}
		if !out[0].StartTime.Equal(overrideStart.UTC()) || !out[0].EndTime.Equal(overrideEnd.UTC()) {
			t.Fatalf("override times not applied: start=%v end=%v", out[0].StartTime, out[0].EndTime)
		}
	})

	t.Run("override moving outside window excludes occurrence", func(t *testing.T) {
		overrideStart := windowEnd.Add(time.Hour)
		overrideEnd := overrideStart.Add(time.Hour)
		exs := []domain.RecurringException{
			{
				SeriesID:        occs[0].SeriesID,
				OccurrenceStart: occs[0].StartTime,
				Kind:            domain.RecurringExceptionKindOverride,
				OverrideStart:   &overrideStart,
				OverrideEnd:     &overrideEnd,
			},
		}

		out := applyRecurringExceptions(occs, exs, windowStart, windowEnd)
		if len(out) != 1 {
			t.Fatalf("len(out) = %d, want 1", len(out))
		}
		if out[0].ID != "2" {
			t.Fatalf("kept occurrence id = %q, want %q", out[0].ID, "2")
		}
	})

	t.Run("only matching occurrence_start is affected", func(t *testing.T) {
		overrideTitle := "new title"
		exs := []domain.RecurringException{
			{
				SeriesID:        occs[0].SeriesID,
				OccurrenceStart: occs[0].StartTime.Add(time.Nanosecond),
				Kind:            domain.RecurringExceptionKindOverride,
				OverrideTitle:   &overrideTitle,
			},
		}

		out := applyRecurringExceptions(occs, exs, windowStart, windowEnd)
		if len(out) != 2 {
			t.Fatalf("len(out) = %d, want 2", len(out))
		}
		if out[0].Title != "t1" {
			t.Fatalf("title = %q, want %q", out[0].Title, "t1")
		}
	})
}

func TestEnsureNoRecurringSeriesConflicts(t *testing.T) {
	baseSeries := func(dtstart time.Time) domain.RecurringSeries {
		until := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
		return domain.RecurringSeries{
			ID:              uuid.MustParse("00000000-0000-0000-0000-000000000201"),
			UserID:          "u1",
			Title:           "t",
			Timezone:        "UTC",
			DTStart:         dtstart,
			DurationSeconds: 3600,
			Frequency:       domain.RecurrenceFrequencyWeekly,
			Interval:        1,
			ByWeekday:       []int16{1},
			Until:           &until,
		}
	}

	t.Run("conflicts with existing appointments detected", func(t *testing.T) {
		series := baseSeries(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))

		tx := &fakeCalendarTx{
			listAppointmentsFn: func(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error) {
				return []domain.Appointment{
					{
						ID:        uuid.MustParse("00000000-0000-0000-0000-000000000301"),
						UserID:    userID,
						Title:     "existing",
						StartTime: time.Date(2026, 1, 12, 9, 30, 0, 0, time.UTC),
						EndTime:   time.Date(2026, 1, 12, 9, 45, 0, 0, time.UTC),
					},
				}, nil
			},
		}

		err := ensureNoRecurringSeriesConflicts(context.Background(), tx, series)
		if err != store.ErrConflict {
			t.Fatalf("err = %v, want %v", err, store.ErrConflict)
		}
	})

	t.Run("conflicts with existing series occurrences detected", func(t *testing.T) {
		series := baseSeries(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))

		existingSeries := baseSeries(time.Date(2026, 1, 5, 9, 30, 0, 0, time.UTC))
		existingSeries.ID = uuid.MustParse("00000000-0000-0000-0000-000000000202")

		tx := &fakeCalendarTx{
			listRecurringSeriesFn: func(ctx context.Context, userID string) ([]domain.RecurringSeries, error) {
				return []domain.RecurringSeries{existingSeries}, nil
			},
		}

		err := ensureNoRecurringSeriesConflicts(context.Background(), tx, series)
		if err != store.ErrConflict {
			t.Fatalf("err = %v, want %v", err, store.ErrConflict)
		}
	})

	t.Run("exceptions are applied before checking overlaps", func(t *testing.T) {
		series := baseSeries(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))

		existingSeries := baseSeries(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))
		existingSeries.ID = uuid.MustParse("00000000-0000-0000-0000-000000000203")
		until := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
		existingSeries.Until = &until

		conflictingOccurrenceStart := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

		tx := &fakeCalendarTx{
			listRecurringSeriesFn: func(ctx context.Context, userID string) ([]domain.RecurringSeries, error) {
				return []domain.RecurringSeries{existingSeries}, nil
			},
			listRecurringExceptionsFn: func(ctx context.Context, seriesID uuid.UUID, windowStart, windowEnd time.Time) ([]domain.RecurringException, error) {
				if seriesID != existingSeries.ID {
					return nil, nil
				}
				return []domain.RecurringException{
					{
						SeriesID:        existingSeries.ID,
						OccurrenceStart: conflictingOccurrenceStart,
						Kind:            domain.RecurringExceptionKindSkip,
					},
				}, nil
			},
		}

		err := ensureNoRecurringSeriesConflicts(context.Background(), tx, series)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})

	t.Run("no false positives when times do not overlap", func(t *testing.T) {
		series := baseSeries(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))

		tx := &fakeCalendarTx{
			listAppointmentsFn: func(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error) {
				return []domain.Appointment{
					{
						ID:        uuid.MustParse("00000000-0000-0000-0000-000000000302"),
						UserID:    userID,
						Title:     "non-overlapping",
						StartTime: time.Date(2026, 1, 12, 10, 0, 0, 0, time.UTC),
						EndTime:   time.Date(2026, 1, 12, 11, 0, 0, 0, time.UTC),
					},
				}, nil
			},
		}

		err := ensureNoRecurringSeriesConflicts(context.Background(), tx, series)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})
}
