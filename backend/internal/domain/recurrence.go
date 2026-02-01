package domain

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type RecurrenceFrequency string

const (
	RecurrenceFrequencyWeekly RecurrenceFrequency = "weekly"
)

type RecurringSeries struct {
	bun.BaseModel `bun:"table:recurring_series"`

	ID              uuid.UUID           `bun:"id,pk,type:uuid"`
	UserID          string              `bun:"user_id,notnull"`
	Title           string              `bun:"title,notnull"`
	Notes           string              `bun:"notes"`
	Timezone        string              `bun:"timezone,notnull"`
	DTStart         time.Time           `bun:"dtstart,notnull"`
	DurationSeconds int                 `bun:"duration_seconds,notnull"`
	Frequency       RecurrenceFrequency `bun:"frequency,notnull"`
	Interval        int                 `bun:"interval,notnull"`
	ByWeekday       []int16             `bun:"byweekday,array,notnull"`
	Until           *time.Time          `bun:"until"`
	Count           *int                `bun:"count"`
	CreatedAt       time.Time           `bun:"created_at,notnull"`
	UpdatedAt       time.Time           `bun:"updated_at,notnull"`
}

func (s *RecurringSeries) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	now := time.Now().UTC()
	switch query.(type) {
	case *bun.InsertQuery:
		if s.ID == uuid.Nil {
			id, err := uuid.NewV7()
			if err != nil {
				return err
			}
			s.ID = id
		}
		if s.CreatedAt.IsZero() {
			s.CreatedAt = now
		}
		if s.UpdatedAt.IsZero() {
			s.UpdatedAt = now
		}
	case *bun.UpdateQuery:
		s.UpdatedAt = now
	}
	return nil
}

type RecurringExceptionKind string

const (
	RecurringExceptionKindSkip     RecurringExceptionKind = "skip"
	RecurringExceptionKindOverride RecurringExceptionKind = "override"
)

type RecurringException struct {
	bun.BaseModel `bun:"table:recurring_exceptions"`

	ID              uuid.UUID              `bun:"id,pk,type:uuid"`
	SeriesID        uuid.UUID              `bun:"series_id,notnull,type:uuid"`
	OccurrenceStart time.Time              `bun:"occurrence_start,notnull"`
	Kind            RecurringExceptionKind `bun:"kind,notnull"`
	OverrideStart   *time.Time             `bun:"override_start"`
	OverrideEnd     *time.Time             `bun:"override_end"`
	OverrideTitle   *string                `bun:"override_title"`
	OverrideNotes   *string                `bun:"override_notes"`
	CreatedAt       time.Time              `bun:"created_at,notnull"`
	UpdatedAt       time.Time              `bun:"updated_at,notnull"`
}

func (e *RecurringException) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	now := time.Now().UTC()
	switch query.(type) {
	case *bun.InsertQuery:
		if e.ID == uuid.Nil {
			id, err := uuid.NewV7()
			if err != nil {
				return err
			}
			e.ID = id
		}
		if e.CreatedAt.IsZero() {
			e.CreatedAt = now
		}
		if e.UpdatedAt.IsZero() {
			e.UpdatedAt = now
		}
	case *bun.UpdateQuery:
		e.UpdatedAt = now
	}
	return nil
}

type RecurringOccurrence struct {
	ID        string
	SeriesID  uuid.UUID
	UserID    string
	Title     string
	Notes     string
	StartTime time.Time
	EndTime   time.Time
}

func GenerateWeeklyOccurrences(series RecurringSeries, windowStart, windowEnd time.Time) ([]RecurringOccurrence, error) {
	if series.Frequency != RecurrenceFrequencyWeekly {
		return nil, errors.New("unsupported recurrence frequency")
	}
	if series.DurationSeconds <= 0 {
		return nil, errors.New("invalid duration")
	}

	loc, err := time.LoadLocation(series.Timezone)
	if err != nil {
		return nil, errors.New("invalid time_zone")
	}

	dtstartUTC := series.DTStart.UTC()
	dtstartLocal := series.DTStart.In(loc)
	duration := time.Duration(series.DurationSeconds) * time.Second

	weekdays := make([]int16, 0, len(series.ByWeekday))
	seen := make(map[int16]struct{}, len(series.ByWeekday))
	for _, wd := range series.ByWeekday {
		if wd < 1 || wd > 7 {
			return nil, errors.New("invalid weekday")
		}
		if _, ok := seen[wd]; ok {
			continue
		}
		seen[wd] = struct{}{}
		weekdays = append(weekdays, wd)
	}
	if len(weekdays) == 0 {
		return nil, errors.New("at least one weekday is required")
	}
	sort.Slice(weekdays, func(i, j int) bool { return weekdays[i] < weekdays[j] })

	interval := series.Interval
	if interval < 1 {
		interval = 1
	}

	windowStartLocal := windowStart.In(loc)
	windowEndLocal := windowEnd.In(loc)
	startWeekMondayUTC := mondayDateUTC(dtstartLocal)
	windowStartWeekMondayUTC := mondayDateUTC(windowStartLocal)
	windowEndWeekBoundaryUTC := mondayDateUTC(windowEndLocal).AddDate(0, 0, 7)

	startWeekIndex := 0
	if windowStartWeekMondayUTC.After(startWeekMondayUTC) {
		daysDiff := int(windowStartWeekMondayUTC.Sub(startWeekMondayUTC) / (24 * time.Hour))
		startWeekIndex = daysDiff / (7 * interval)
		if startWeekIndex < 0 {
			startWeekIndex = 0
		}
	}

	maxCount := -1
	if series.Count != nil {
		maxCount = *series.Count
	}

	occPerWeek := len(weekdays)
	skippedInFirstWeek := 0
	for _, wd := range weekdays {
		occDateUTC := startWeekMondayUTC.AddDate(0, 0, weekdayOffsetFromMonday(wd))
		startLocal := time.Date(
			occDateUTC.Year(),
			occDateUTC.Month(),
			occDateUTC.Day(),
			dtstartLocal.Hour(),
			dtstartLocal.Minute(),
			dtstartLocal.Second(),
			dtstartLocal.Nanosecond(),
			loc,
		)
		if startLocal.UTC().Before(dtstartUTC) {
			skippedInFirstWeek++
		}
	}

	out := make([]RecurringOccurrence, 0, 16)

	for weekIndex := startWeekIndex; ; weekIndex++ {
		weekStartMondayUTC := startWeekMondayUTC.AddDate(0, 0, weekIndex*interval*7)
		if !weekStartMondayUTC.Before(windowEndWeekBoundaryUTC) {
			break
		}

		for weekdayIndex, wd := range weekdays {
			occDateUTC := weekStartMondayUTC.AddDate(0, 0, weekdayOffsetFromMonday(wd))
			startLocal := time.Date(
				occDateUTC.Year(),
				occDateUTC.Month(),
				occDateUTC.Day(),
				dtstartLocal.Hour(),
				dtstartLocal.Minute(),
				dtstartLocal.Second(),
				dtstartLocal.Nanosecond(),
				loc,
			)
			startUTC := startLocal.UTC()
			if startUTC.Before(dtstartUTC) {
				continue
			}

			if series.Until != nil && startUTC.After(series.Until.UTC()) {
				return out, nil
			}

			if maxCount >= 0 {
				globalIndex := weekIndex*occPerWeek + weekdayIndex - skippedInFirstWeek
				if globalIndex >= maxCount {
					return out, nil
				}
			}

			endUTC := startUTC.Add(duration)
			if startUTC.Before(windowEnd) && endUTC.After(windowStart) {
				occurrenceID := strconv.FormatInt(startUTC.UnixNano(), 10)
				out = append(out, RecurringOccurrence{
					ID:        occurrenceID,
					SeriesID:  series.ID,
					UserID:    series.UserID,
					Title:     series.Title,
					Notes:     series.Notes,
					StartTime: startUTC,
					EndTime:   endUTC,
				})
			}
		}
	}

	return out, nil
}

func mondayDateUTC(t time.Time) time.Time {
	wd := t.Weekday()
	offset := 0
	if wd == time.Sunday {
		offset = 6
	} else {
		offset = int(wd) - 1
	}
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return d.AddDate(0, 0, -offset)
}

func weekdayOffsetFromMonday(weekday int16) int {
	if weekday == 7 {
		return 6
	}
	return int(weekday) - 1
}
