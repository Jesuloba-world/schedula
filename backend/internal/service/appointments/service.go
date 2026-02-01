package appointments

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"schedula/backend/internal/domain"
	"schedula/backend/internal/store"
)

type ValidationError struct {
	msg string
}

func (e *ValidationError) Error() string {
	return e.msg
}

func validationError(msg string) error {
	return &ValidationError{msg: msg}
}

type Service struct {
	repo store.AppointmentRepository
}

func NewService(repo store.AppointmentRepository) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	UserID         string
	Title          string
	Notes          string
	StartTime      time.Time
	EndTime        time.Time
	IdempotencyKey string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (domain.Appointment, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return domain.Appointment{}, validationError("title is required")
	}
	if in.UserID == "" {
		return domain.Appointment{}, validationError("user_id is required")
	}

	start := in.StartTime.UTC()
	end := in.EndTime.UTC()
	if end.Equal(start) || end.Before(start) {
		return domain.Appointment{}, validationError("end_time must be after start_time")
	}
	if end.Sub(start) > 24*time.Hour {
		return domain.Appointment{}, validationError("duration too long")
	}

	appt := domain.Appointment{
		UserID:    in.UserID,
		Title:     title,
		Notes:     in.Notes,
		StartTime: start,
		EndTime:   end,
	}

	key := strings.TrimSpace(in.IdempotencyKey)
	if key != "" {
		if len(key) > 256 {
			return domain.Appointment{}, validationError("idempotency_key too long")
		}
		appt.ID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("schedula:create_appointment:"+in.UserID+":"+key))
	}

	return s.repo.Create(ctx, appt)
}

func (s *Service) List(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error) {
	if userID == "" {
		return nil, validationError("user_id is required")
	}

	start := windowStart.UTC()
	end := windowEnd.UTC()
	if end.Equal(start) || end.Before(start) {
		return nil, validationError("window_end must be after window_start")
	}

	return s.repo.List(ctx, userID, start, end)
}

func (s *Service) Delete(ctx context.Context, userID string, appointmentID uuid.UUID) error {
	if userID == "" {
		return validationError("user_id is required")
	}
	if appointmentID == uuid.Nil {
		return validationError("appointment_id is required")
	}
	return s.repo.Delete(ctx, userID, appointmentID)
}

type CreateRecurringSeriesInput struct {
	UserID    string
	Title     string
	Notes     string
	StartTime time.Time
	EndTime   time.Time
	Rule      RecurrenceRuleInput
}

type RecurrenceRuleInput struct {
	Frequency domain.RecurrenceFrequency
	Interval  int
	ByWeekday []int16
	Until     *time.Time
	Count     *int
	TimeZone  string
}

func (s *Service) CreateRecurringSeries(ctx context.Context, in CreateRecurringSeriesInput) (domain.RecurringSeries, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return domain.RecurringSeries{}, validationError("title is required")
	}
	if in.UserID == "" {
		return domain.RecurringSeries{}, validationError("user_id is required")
	}

	frequency := in.Rule.Frequency
	if frequency == "" {
		frequency = domain.RecurrenceFrequencyWeekly
	}
	if frequency != domain.RecurrenceFrequencyWeekly {
		return domain.RecurringSeries{}, validationError("unsupported frequency")
	}

	tz := strings.TrimSpace(in.Rule.TimeZone)
	if tz == "" {
		return domain.RecurringSeries{}, validationError("time_zone is required")
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return domain.RecurringSeries{}, validationError("invalid time_zone")
	}

	start := in.StartTime.UTC()
	end := in.EndTime.UTC()
	if end.Equal(start) || end.Before(start) {
		return domain.RecurringSeries{}, validationError("end_time must be after start_time")
	}
	if end.Sub(start) > 24*time.Hour {
		return domain.RecurringSeries{}, validationError("duration too long")
	}
	durationSeconds := int(end.Sub(start) / time.Second)

	interval := in.Rule.Interval
	if interval == 0 {
		interval = 1
	}
	if interval < 1 {
		return domain.RecurringSeries{}, validationError("interval must be at least 1")
	}

	weekdays := in.Rule.ByWeekday
	if len(weekdays) == 0 {
		weekday := start.In(loc).Weekday()
		if weekday == time.Sunday {
			weekdays = []int16{7}
		} else {
			weekdays = []int16{int16(weekday)}
		}
	}

	dedup := make(map[int16]struct{}, len(weekdays))
	normalized := make([]int16, 0, len(weekdays))
	for _, wd := range weekdays {
		if wd < 1 || wd > 7 {
			return domain.RecurringSeries{}, validationError("invalid weekday")
		}
		if _, ok := dedup[wd]; ok {
			continue
		}
		dedup[wd] = struct{}{}
		normalized = append(normalized, wd)
	}
	if len(normalized) == 0 {
		return domain.RecurringSeries{}, validationError("at least one weekday is required")
	}

	for i := 1; i < len(normalized); i++ {
		key := normalized[i]
		j := i - 1
		for j >= 0 && normalized[j] > key {
			normalized[j+1] = normalized[j]
			j--
		}
		normalized[j+1] = key
	}

	var untilUTC *time.Time
	if in.Rule.Until != nil {
		u := in.Rule.Until.UTC()
		if u.Before(start) {
			return domain.RecurringSeries{}, validationError("until must be after start_time")
		}
		untilUTC = &u
	}

	var count *int
	if in.Rule.Count != nil {
		c := *in.Rule.Count
		if c < 1 {
			return domain.RecurringSeries{}, validationError("count must be at least 1")
		}
		count = &c
	}

	if untilUTC == nil && count == nil {
		return domain.RecurringSeries{}, validationError("until or count is required")
	}

	series := domain.RecurringSeries{
		UserID:          in.UserID,
		Title:           title,
		Notes:           in.Notes,
		Timezone:        tz,
		DTStart:         start,
		DurationSeconds: durationSeconds,
		Frequency:       frequency,
		Interval:        interval,
		ByWeekday:       normalized,
		Until:           untilUTC,
		Count:           count,
	}

	lookaheadEnd := start.Add(store.RecurringConflictLookahead)
	duration := time.Duration(durationSeconds) * time.Second

	if count == nil {
		if untilUTC != nil && untilUTC.After(lookaheadEnd) {
			return domain.RecurringSeries{}, validationError("until must be within 180 days of start_time")
		}
	}

	occLimitEnd := lookaheadEnd
	if untilUTC != nil && untilUTC.Before(occLimitEnd) {
		occLimitEnd = *untilUTC
	}

	seriesForCount := series
	seriesForCount.Until = &occLimitEnd
	seriesForCount.Count = nil
	occs, err := domain.GenerateWeeklyOccurrences(seriesForCount, start, occLimitEnd.Add(duration))
	if err != nil {
		return domain.RecurringSeries{}, err
	}
	if len(occs) == 0 {
		return domain.RecurringSeries{}, validationError("recurrence rule produces no occurrences")
	}
	if count != nil && *count > len(occs) {
		if untilUTC != nil && untilUTC.Before(lookaheadEnd) {
			return domain.RecurringSeries{}, validationError("count exceeds occurrences available before until")
		}
		return domain.RecurringSeries{}, validationError("count exceeds occurrences available within 180 days of start_time")
	}

	return s.repo.CreateRecurringSeries(ctx, series)
}

func (s *Service) ListOccurrences(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.RecurringOccurrence, error) {
	if userID == "" {
		return nil, validationError("user_id is required")
	}

	start := windowStart.UTC()
	end := windowEnd.UTC()
	if end.Equal(start) || end.Before(start) {
		return nil, validationError("window_end must be after window_start")
	}

	return s.repo.ListOccurrences(ctx, userID, start, end)
}
