package postgres

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/uptrace/bun"

	"schedula/backend/internal/domain"
	"schedula/backend/internal/store"
)

type AppointmentRepo struct {
	db *bun.DB
}

func NewAppointmentRepo(db *bun.DB) *AppointmentRepo {
	return &AppointmentRepo{db: db}
}

type calendarTx struct {
	tx bun.Tx
}

func (r *AppointmentRepo) Create(ctx context.Context, appt domain.Appointment) (domain.Appointment, error) {
	var out domain.Appointment
	err := r.InUserTransaction(ctx, appt.UserID, func(ctx context.Context, tx store.CalendarTx) error {
		a, err := tx.CreateAppointment(ctx, appt)
		if err != nil {
			return err
		}
		out = a
		return nil
	})
	if err != nil {
		return domain.Appointment{}, err
	}
	return out, nil
}

func (r *AppointmentRepo) List(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error) {
	var rows []domain.Appointment
	err := r.db.NewSelect().
		Model(&rows).
		Where("user_id = ?", userID).
		Where("start_time < ?", windowEnd).
		Where("end_time > ?", windowStart).
		OrderExpr("start_time ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *AppointmentRepo) Delete(ctx context.Context, userID string, appointmentID uuid.UUID) error {
	return r.InUserTransaction(ctx, userID, func(ctx context.Context, tx store.CalendarTx) error {
		return tx.DeleteAppointment(ctx, userID, appointmentID)
	})
}

func (r *AppointmentRepo) CreateRecurringSeries(ctx context.Context, series domain.RecurringSeries) (domain.RecurringSeries, error) {
	var out domain.RecurringSeries
	err := r.InUserTransaction(ctx, series.UserID, func(ctx context.Context, tx store.CalendarTx) error {
		if err := ensureNoRecurringSeriesConflicts(ctx, tx, series); err != nil {
			return err
		}
		s, err := tx.CreateRecurringSeries(ctx, series)
		if err != nil {
			return err
		}
		out = s
		return nil
	})
	if err != nil {
		return domain.RecurringSeries{}, err
	}
	return out, nil
}

func (r *AppointmentRepo) ListOccurrences(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.RecurringOccurrence, error) {
	var seriesRows []domain.RecurringSeries
	err := r.db.NewSelect().
		Model(&seriesRows).
		Where("user_id = ?", userID).
		Where("dtstart < ?", windowEnd).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]domain.RecurringOccurrence, 0, len(seriesRows))
	exWindowStart := windowStart.Add(-14 * 24 * time.Hour)
	exWindowEnd := windowEnd.Add(14 * 24 * time.Hour)

	for _, s := range seriesRows {
		occs, err := domain.GenerateWeeklyOccurrences(s, windowStart, windowEnd)
		if err != nil {
			return nil, err
		}
		if len(occs) == 0 {
			continue
		}

		var exRows []domain.RecurringException
		err = r.db.NewSelect().
			Model(&exRows).
			Where("series_id = ?", s.ID).
			Where("occurrence_start >= ?", exWindowStart).
			Where("occurrence_start < ?", exWindowEnd).
			Scan(ctx)
		if err != nil {
			return nil, err
		}

		out = append(out, applyRecurringExceptions(occs, exRows, windowStart, windowEnd)...)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].StartTime.Before(out[j].StartTime)
	})

	return out, nil
}

func (r *AppointmentRepo) InUserTransaction(ctx context.Context, userID string, fn func(ctx context.Context, tx store.CalendarTx) error) error {
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockUserCalendar(ctx, tx, userID); err != nil {
			return err
		}
		return fn(ctx, calendarTx{tx: tx})
	})
}

func lockUserCalendar(ctx context.Context, tx bun.Tx, userID string) error {
	_, err := tx.NewRaw("SELECT pg_advisory_xact_lock(hashtext(?))", userID).Exec(ctx)
	return err
}

func (r calendarTx) CreateAppointment(ctx context.Context, appt domain.Appointment) (domain.Appointment, error) {
	m := domain.Appointment{
		ID:        appt.ID,
		UserID:    appt.UserID,
		Title:     appt.Title,
		Notes:     appt.Notes,
		StartTime: appt.StartTime,
		EndTime:   appt.EndTime,
		CreatedAt: appt.CreatedAt,
		UpdatedAt: appt.UpdatedAt,
	}

	_, err := r.tx.NewInsert().Model(&m).Exec(ctx)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23P01" && pgErr.ConstraintName == "appointments_no_overlap" {
				return domain.Appointment{}, store.ErrConflict
			}
			if pgErr.Code == "23505" {
				var existing domain.Appointment
				selectErr := r.tx.NewSelect().
					Model(&existing).
					Where("id = ?", m.ID).
					Limit(1).
					Scan(ctx)
				if selectErr != nil {
					return domain.Appointment{}, err
				}

				if existing.UserID != appt.UserID ||
					existing.Title != appt.Title ||
					existing.Notes != appt.Notes ||
					!existing.StartTime.Equal(appt.StartTime) ||
					!existing.EndTime.Equal(appt.EndTime) {
					return domain.Appointment{}, store.ErrIdempotencyConflict
				}

				return existing, nil
			}
		}
		return domain.Appointment{}, err
	}

	appt.ID = m.ID
	return appt, nil
}

func (r calendarTx) ListAppointments(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error) {
	var rows []domain.Appointment
	err := r.tx.NewSelect().
		Model(&rows).
		Where("user_id = ?", userID).
		Where("start_time < ?", windowEnd).
		Where("end_time > ?", windowStart).
		OrderExpr("start_time ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r calendarTx) DeleteAppointment(ctx context.Context, userID string, appointmentID uuid.UUID) error {
	res, err := r.tx.NewDelete().
		Model((*domain.Appointment)(nil)).
		Where("user_id = ?", userID).
		Where("id = ?", appointmentID).
		Exec(ctx)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (r calendarTx) CreateRecurringSeries(ctx context.Context, series domain.RecurringSeries) (domain.RecurringSeries, error) {
	m := domain.RecurringSeries{
		ID:              series.ID,
		UserID:          series.UserID,
		Title:           series.Title,
		Notes:           series.Notes,
		Timezone:        series.Timezone,
		DTStart:         series.DTStart,
		DurationSeconds: series.DurationSeconds,
		Frequency:       series.Frequency,
		Interval:        series.Interval,
		ByWeekday:       series.ByWeekday,
		Until:           series.Until,
		Count:           series.Count,
		CreatedAt:       series.CreatedAt,
		UpdatedAt:       series.UpdatedAt,
	}

	_, err := r.tx.NewInsert().Model(&m).Exec(ctx)
	if err != nil {
		return domain.RecurringSeries{}, err
	}
	series.ID = m.ID
	return series, nil
}

func (r calendarTx) ListRecurringSeries(ctx context.Context, userID string) ([]domain.RecurringSeries, error) {
	var rows []domain.RecurringSeries
	err := r.tx.NewSelect().
		Model(&rows).
		Where("user_id = ?", userID).
		OrderExpr("dtstart ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r calendarTx) ListRecurringExceptions(ctx context.Context, seriesID uuid.UUID, windowStart, windowEnd time.Time) ([]domain.RecurringException, error) {
	var rows []domain.RecurringException
	err := r.tx.NewSelect().
		Model(&rows).
		Where("series_id = ?", seriesID).
		Where("occurrence_start >= ?", windowStart).
		Where("occurrence_start < ?", windowEnd).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r calendarTx) UpsertRecurringException(ctx context.Context, ex domain.RecurringException) (domain.RecurringException, error) {
	m := domain.RecurringException{
		ID:              ex.ID,
		SeriesID:        ex.SeriesID,
		OccurrenceStart: ex.OccurrenceStart,
		Kind:            ex.Kind,
		OverrideStart:   ex.OverrideStart,
		OverrideEnd:     ex.OverrideEnd,
		OverrideTitle:   ex.OverrideTitle,
		OverrideNotes:   ex.OverrideNotes,
		CreatedAt:       ex.CreatedAt,
		UpdatedAt:       ex.UpdatedAt,
	}

	_, err := r.tx.NewInsert().
		Model(&m).
		On("CONFLICT (series_id, occurrence_start) DO UPDATE").
		Set("kind = EXCLUDED.kind").
		Set("override_start = EXCLUDED.override_start").
		Set("override_end = EXCLUDED.override_end").
		Set("override_title = EXCLUDED.override_title").
		Set("override_notes = EXCLUDED.override_notes").
		Exec(ctx)
	if err != nil {
		return domain.RecurringException{}, err
	}
	return m, nil
}

func (r calendarTx) DeleteRecurringSeries(ctx context.Context, userID string, seriesID uuid.UUID) error {
	res, err := r.tx.NewDelete().
		Model((*domain.RecurringSeries)(nil)).
		Where("user_id = ?", userID).
		Where("id = ?", seriesID).
		Exec(ctx)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return store.ErrNotFound
	}
	return nil
}

type timeSpan struct {
	Start time.Time
	End   time.Time
}

func ensureNoRecurringSeriesConflicts(ctx context.Context, tx store.CalendarTx, series domain.RecurringSeries) error {
	windowStart := series.DTStart.UTC()
	windowEnd := windowStart.Add(store.RecurringConflictLookahead)
	if series.Until != nil && series.Until.UTC().Before(windowEnd) {
		windowEnd = series.Until.UTC()
	}
	windowEnd = windowEnd.Add(time.Duration(series.DurationSeconds) * time.Second)

	newOccs, err := domain.GenerateWeeklyOccurrences(series, windowStart, windowEnd)
	if err != nil {
		return err
	}
	if len(newOccs) == 0 {
		return nil
	}
	sort.Slice(newOccs, func(i, j int) bool {
		return newOccs[i].StartTime.Before(newOccs[j].StartTime)
	})
	windowEnd = newOccs[len(newOccs)-1].EndTime.UTC()

	for i := 1; i < len(newOccs); i++ {
		if newOccs[i-1].EndTime.After(newOccs[i].StartTime) {
			return store.ErrConflict
		}
	}

	appts, err := tx.ListAppointments(ctx, series.UserID, windowStart, windowEnd)
	if err != nil {
		return err
	}

	existing := make([]timeSpan, 0, len(appts))
	for _, a := range appts {
		existing = append(existing, timeSpan{Start: a.StartTime.UTC(), End: a.EndTime.UTC()})
	}

	seriesRows, err := tx.ListRecurringSeries(ctx, series.UserID)
	if err != nil {
		return err
	}

	exWindowStart := windowStart.Add(-14 * 24 * time.Hour)
	exWindowEnd := windowEnd.Add(14 * 24 * time.Hour)

	for _, s := range seriesRows {
		occs, err := domain.GenerateWeeklyOccurrences(s, windowStart, windowEnd)
		if err != nil {
			return err
		}
		if len(occs) == 0 {
			continue
		}

		exRows, err := tx.ListRecurringExceptions(ctx, s.ID, exWindowStart, exWindowEnd)
		if err != nil {
			return err
		}

		occs = applyRecurringExceptions(occs, exRows, windowStart, windowEnd)
		for _, o := range occs {
			existing = append(existing, timeSpan{Start: o.StartTime.UTC(), End: o.EndTime.UTC()})
		}
	}

	for _, n := range newOccs {
		ns := n.StartTime.UTC()
		ne := n.EndTime.UTC()
		for _, e := range existing {
			if ns.Before(e.End) && ne.After(e.Start) {
				return store.ErrConflict
			}
		}
	}

	return nil
}

func applyRecurringExceptions(occs []domain.RecurringOccurrence, exs []domain.RecurringException, windowStart, windowEnd time.Time) []domain.RecurringOccurrence {
	if len(exs) == 0 {
		return occs
	}

	byOccurrenceStart := make(map[int64]domain.RecurringException, len(exs))
	for _, e := range exs {
		byOccurrenceStart[e.OccurrenceStart.UTC().UnixNano()] = e
	}

	out := make([]domain.RecurringOccurrence, 0, len(occs))
	for _, o := range occs {
		ex, ok := byOccurrenceStart[o.StartTime.UTC().UnixNano()]
		if !ok {
			out = append(out, o)
			continue
		}

		if ex.Kind == domain.RecurringExceptionKindSkip {
			continue
		}

		start := o.StartTime
		end := o.EndTime
		title := o.Title
		notes := o.Notes

		if ex.OverrideStart != nil {
			start = ex.OverrideStart.UTC()
		}
		if ex.OverrideEnd != nil {
			end = ex.OverrideEnd.UTC()
		}
		if ex.OverrideTitle != nil {
			title = *ex.OverrideTitle
		}
		if ex.OverrideNotes != nil {
			notes = *ex.OverrideNotes
		}

		if start.Before(windowEnd) && end.After(windowStart) {
			out = append(out, domain.RecurringOccurrence{
				ID:        o.ID,
				SeriesID:  o.SeriesID,
				UserID:    o.UserID,
				Title:     title,
				Notes:     notes,
				StartTime: start,
				EndTime:   end,
			})
		}
	}

	return out
}
