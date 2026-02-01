package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateWeeklyOccurrences_Validation(t *testing.T) {
	base := RecurringSeries{
		ID:              uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		UserID:          "u1",
		Title:           "title",
		Timezone:        "UTC",
		DTStart:         time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC),
		DurationSeconds: 3600,
		Frequency:       RecurrenceFrequencyWeekly,
		Interval:        1,
		ByWeekday:       []int16{1},
	}

	windowStart := base.DTStart
	windowEnd := windowStart.Add(7 * 24 * time.Hour)

	tests := []struct {
		name    string
		series  RecurringSeries
		wantErr string
	}{
		{
			name: "unsupported frequency",
			series: func() RecurringSeries {
				s := base
				s.Frequency = "daily"
				return s
			}(),
			wantErr: "unsupported recurrence frequency",
		},
		{
			name: "invalid duration",
			series: func() RecurringSeries {
				s := base
				s.DurationSeconds = 0
				return s
			}(),
			wantErr: "invalid duration",
		},
		{
			name: "invalid time zone",
			series: func() RecurringSeries {
				s := base
				s.Timezone = "Not/AZone"
				return s
			}(),
			wantErr: "invalid time_zone",
		},
		{
			name: "invalid weekday",
			series: func() RecurringSeries {
				s := base
				s.ByWeekday = []int16{0}
				return s
			}(),
			wantErr: "invalid weekday",
		},
		{
			name: "empty weekday set",
			series: func() RecurringSeries {
				s := base
				s.ByWeekday = nil
				return s
			}(),
			wantErr: "at least one weekday is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GenerateWeeklyOccurrences(tt.series, windowStart, windowEnd)
			if err == nil {
				t.Fatalf("expected error")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGenerateWeeklyOccurrences_NormalizesIntervalAndWeekdays(t *testing.T) {
	series := RecurringSeries{
		ID:              uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		UserID:          "u1",
		Title:           "title",
		Timezone:        "UTC",
		DTStart:         time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC),
		DurationSeconds: 3600,
		Frequency:       RecurrenceFrequencyWeekly,
		Interval:        0,
		ByWeekday:       []int16{3, 1, 3},
	}

	windowStart := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC)

	occs, err := GenerateWeeklyOccurrences(series, windowStart, windowEnd)
	if err != nil {
		t.Fatalf("GenerateWeeklyOccurrences error: %v", err)
	}
	if len(occs) != 4 {
		t.Fatalf("len(occs) = %d, want 4", len(occs))
	}
	for i := 1; i < len(occs); i++ {
		if !occs[i-1].StartTime.Before(occs[i].StartTime) {
			t.Fatalf("occurrences not sorted by start_time: %v then %v", occs[i-1].StartTime, occs[i].StartTime)
		}
	}
}

func TestGenerateWeeklyOccurrences_IncludesWindowOverlap(t *testing.T) {
	series := RecurringSeries{
		ID:              uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		UserID:          "u1",
		Title:           "title",
		Timezone:        "UTC",
		DTStart:         time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC),
		DurationSeconds: int((2 * time.Hour) / time.Second),
		Frequency:       RecurrenceFrequencyWeekly,
		Interval:        1,
		ByWeekday:       []int16{1},
	}

	windowStart := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 1, 5, 10, 30, 0, 0, time.UTC)

	occs, err := GenerateWeeklyOccurrences(series, windowStart, windowEnd)
	if err != nil {
		t.Fatalf("GenerateWeeklyOccurrences error: %v", err)
	}
	if len(occs) != 1 {
		t.Fatalf("len(occs) = %d, want 1", len(occs))
	}
	if !occs[0].StartTime.Before(windowEnd) || !occs[0].EndTime.After(windowStart) {
		t.Fatalf("occurrence does not overlap window: start=%v end=%v", occs[0].StartTime, occs[0].EndTime)
	}
}

func TestGenerateWeeklyOccurrences_RespectsUntilAndCount(t *testing.T) {
	until := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	count := 2

	series := RecurringSeries{
		ID:              uuid.MustParse("00000000-0000-0000-0000-000000000004"),
		UserID:          "u1",
		Title:           "title",
		Timezone:        "UTC",
		DTStart:         time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC),
		DurationSeconds: 3600,
		Frequency:       RecurrenceFrequencyWeekly,
		Interval:        1,
		ByWeekday:       []int16{1},
		Until:           &until,
		Count:           &count,
	}

	windowStart := series.DTStart
	windowEnd := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	occs, err := GenerateWeeklyOccurrences(series, windowStart, windowEnd)
	if err != nil {
		t.Fatalf("GenerateWeeklyOccurrences error: %v", err)
	}
	if len(occs) != 2 {
		t.Fatalf("len(occs) = %d, want 2", len(occs))
	}
}

func TestGenerateWeeklyOccurrences_DSTMaintainsLocalHour(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation error: %v", err)
	}

	dtstartLocal := time.Date(2026, 3, 1, 9, 0, 0, 0, loc)

	series := RecurringSeries{
		ID:              uuid.MustParse("00000000-0000-0000-0000-000000000005"),
		UserID:          "u1",
		Title:           "title",
		Timezone:        "America/New_York",
		DTStart:         dtstartLocal,
		DurationSeconds: 3600,
		Frequency:       RecurrenceFrequencyWeekly,
		Interval:        1,
		ByWeekday:       []int16{7},
	}

	windowStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC)

	occs, err := GenerateWeeklyOccurrences(series, windowStart, windowEnd)
	if err != nil {
		t.Fatalf("GenerateWeeklyOccurrences error: %v", err)
	}
	if len(occs) == 0 {
		t.Fatalf("expected occurrences")
	}

	for _, o := range occs {
		if o.StartTime.In(loc).Hour() != 9 {
			t.Fatalf("local hour = %d, want 9 (start_time=%v)", o.StartTime.In(loc).Hour(), o.StartTime)
		}
		if !o.StartTime.Before(o.EndTime) {
			t.Fatalf("start_time must be before end_time: %v %v", o.StartTime, o.EndTime)
		}
		if !o.StartTime.Before(windowEnd) || !o.EndTime.After(windowStart) {
			t.Fatalf("occurrence does not overlap window: %v %v", o.StartTime, o.EndTime)
		}
	}
}

