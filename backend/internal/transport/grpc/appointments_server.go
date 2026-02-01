package grpc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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

type AppointmentsServer struct {
	schedulev1.UnimplementedAppointmentsServiceServer

	svc appointmentsService
	log *slog.Logger
}

type appointmentsService interface {
	Create(ctx context.Context, in appointments.CreateInput) (domain.Appointment, error)
	List(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.Appointment, error)
	Delete(ctx context.Context, userID string, appointmentID uuid.UUID) error
	CreateRecurringSeries(ctx context.Context, in appointments.CreateRecurringSeriesInput) (domain.RecurringSeries, error)
	ListOccurrences(ctx context.Context, userID string, windowStart, windowEnd time.Time) ([]domain.RecurringOccurrence, error)
}

func NewAppointmentsServer(svc appointmentsService, log *slog.Logger) *AppointmentsServer {
	if log == nil {
		log = slog.Default()
	}
	return &AppointmentsServer{
		svc: svc,
		log: log.With(slog.String("component", "grpc.appointments")),
	}
}

func (s *AppointmentsServer) CreateAppointment(ctx context.Context, req *schedulev1.CreateAppointmentRequest) (*schedulev1.CreateAppointmentResponse, error) {
	log := s.log.With(slog.String("rpc", "CreateAppointment"))

	if req == nil {
		log.Warn("invalid request", slog.String("reason", "nil_request"))
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.StartTime == nil || req.EndTime == nil {
		log.Warn("invalid request", slog.String("reason", "missing_times"), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.InvalidArgument, "start_time and end_time are required")
	}

	appt, err := s.svc.Create(ctx, appointments.CreateInput{
		UserID:         req.UserId,
		Title:          req.Title,
		Notes:          req.Notes,
		StartTime:      req.StartTime.AsTime(),
		EndTime:        req.EndTime.AsTime(),
		IdempotencyKey: idempotencyKey(ctx),
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			log.Info(
				"appointment create conflict",
				slog.String("user_id", req.UserId),
				slog.Time("start_time", req.StartTime.AsTime()),
				slog.Time("end_time", req.EndTime.AsTime()),
			)
			return nil, status.Error(codes.FailedPrecondition, "You already have an appointment during that time. Pick a different slot.")
		}
		if errors.Is(err, store.ErrIdempotencyConflict) {
			log.Info("appointment create idempotency conflict", slog.String("user_id", req.UserId))
			return nil, status.Error(codes.FailedPrecondition, "This request key was already used for a different appointment. Try again.")
		}
		var vErr *appointments.ValidationError
		if errors.As(err, &vErr) {
			log.Warn("invalid request", slog.Any("err", err), slog.String("user_id", req.UserId))
			return nil, status.Error(codes.InvalidArgument, vErr.Error())
		}
		log.Error("appointment create failed", slog.Any("err", err), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.Internal, "internal error")
	}

	log.Info(
		"appointment created",
		slog.String("appointment_id", appt.ID.String()),
		slog.String("user_id", appt.UserID),
		slog.Time("start_time", appt.StartTime),
		slog.Time("end_time", appt.EndTime),
	)

	return &schedulev1.CreateAppointmentResponse{
		Appointment: toProtoAppointment(appt),
	}, nil
}

func idempotencyKey(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get("idempotency-key")
	if len(values) == 0 {
		values = md.Get("x-idempotency-key")
	}
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func (s *AppointmentsServer) ListAppointments(ctx context.Context, req *schedulev1.ListAppointmentsRequest) (*schedulev1.ListAppointmentsResponse, error) {
	log := s.log.With(slog.String("rpc", "ListAppointments"))

	if req == nil {
		log.Warn("invalid request", slog.String("reason", "nil_request"))
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WindowStart == nil || req.WindowEnd == nil {
		log.Warn("invalid request", slog.String("reason", "missing_window"), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.InvalidArgument, "window_start and window_end are required")
	}

	appts, err := s.svc.List(ctx, req.UserId, req.WindowStart.AsTime(), req.WindowEnd.AsTime())
	if err != nil {
		var vErr *appointments.ValidationError
		if errors.As(err, &vErr) {
			log.Warn("invalid request", slog.Any("err", err), slog.String("user_id", req.UserId))
			return nil, status.Error(codes.InvalidArgument, vErr.Error())
		}
		log.Error("appointments list failed", slog.Any("err", err), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.Internal, "internal error")
	}

	out := make([]*schedulev1.Appointment, 0, len(appts))
	for _, a := range appts {
		out = append(out, toProtoAppointment(a))
	}

	log.Debug(
		"appointments listed",
		slog.String("user_id", req.UserId),
		slog.Int("count", len(out)),
		slog.Time("window_start", req.WindowStart.AsTime()),
		slog.Time("window_end", req.WindowEnd.AsTime()),
	)

	return &schedulev1.ListAppointmentsResponse{Appointments: out}, nil
}

func (s *AppointmentsServer) DeleteAppointment(ctx context.Context, req *schedulev1.DeleteAppointmentRequest) (*schedulev1.DeleteAppointmentResponse, error) {
	log := s.log.With(slog.String("rpc", "DeleteAppointment"))

	if req == nil {
		log.Warn("invalid request", slog.String("reason", "nil_request"))
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	id, err := uuid.Parse(req.AppointmentId)
	if err != nil {
		log.Warn("invalid request", slog.String("reason", "invalid_uuid"), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.InvalidArgument, "appointment_id must be a UUID")
	}

	if err := s.svc.Delete(ctx, req.UserId, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			log.Info("appointment not found", slog.String("appointment_id", id.String()), slog.String("user_id", req.UserId))
			return nil, status.Error(codes.NotFound, "appointment not found")
		}
		var vErr *appointments.ValidationError
		if errors.As(err, &vErr) {
			log.Warn("invalid request", slog.Any("err", err), slog.String("appointment_id", id.String()), slog.String("user_id", req.UserId))
			return nil, status.Error(codes.InvalidArgument, vErr.Error())
		}
		log.Error("appointment delete failed", slog.Any("err", err), slog.String("appointment_id", id.String()), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.Internal, "internal error")
	}

	log.Info("appointment deleted", slog.String("appointment_id", id.String()), slog.String("user_id", req.UserId))
	return &schedulev1.DeleteAppointmentResponse{}, nil
}

func (s *AppointmentsServer) CreateRecurringSeries(ctx context.Context, req *schedulev1.CreateRecurringSeriesRequest) (*schedulev1.CreateRecurringSeriesResponse, error) {
	log := s.log.With(slog.String("rpc", "CreateRecurringSeries"))

	if req == nil {
		log.Warn("invalid request", slog.String("reason", "nil_request"))
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.StartTime == nil || req.EndTime == nil {
		log.Warn("invalid request", slog.String("reason", "missing_times"), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.InvalidArgument, "start_time and end_time are required")
	}
	if req.Weekly == nil {
		log.Warn("invalid request", slog.String("reason", "missing_weekly"), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.InvalidArgument, "weekly is required")
	}

	var until *time.Time
	if req.Weekly.Until != nil {
		u := req.Weekly.Until.AsTime()
		until = &u
	}

	var count *int
	if req.Weekly.Count > 0 {
		c := int(req.Weekly.Count)
		count = &c
	}

	weekdays := make([]int16, 0, len(req.Weekly.Weekdays))
	for _, wd := range req.Weekly.Weekdays {
		if wd == schedulev1.Weekday_WEEKDAY_UNSPECIFIED {
			continue
		}
		weekdays = append(weekdays, int16(wd))
	}

	series, err := s.svc.CreateRecurringSeries(ctx, appointments.CreateRecurringSeriesInput{
		UserID:    req.UserId,
		Title:     req.Title,
		Notes:     req.Notes,
		StartTime: req.StartTime.AsTime(),
		EndTime:   req.EndTime.AsTime(),
		Rule: appointments.RecurrenceRuleInput{
			Frequency: domain.RecurrenceFrequencyWeekly,
			Interval:  int(req.Weekly.Interval),
			ByWeekday: weekdays,
			Until:     until,
			Count:     count,
			TimeZone:  req.Weekly.TimeZone,
		},
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			log.Info(
				"recurring series create conflict",
				slog.String("user_id", req.UserId),
				slog.Time("start_time", req.StartTime.AsTime()),
				slog.Time("end_time", req.EndTime.AsTime()),
			)
			return nil, status.Error(codes.FailedPrecondition, "You already have an appointment during that time. Pick a different slot.")
		}
		var vErr *appointments.ValidationError
		if errors.As(err, &vErr) {
			log.Warn("invalid request", slog.Any("err", err), slog.String("user_id", req.UserId))
			return nil, status.Error(codes.InvalidArgument, vErr.Error())
		}
		log.Error("recurring series create failed", slog.Any("err", err), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.Internal, "internal error")
	}

	log.Info(
		"recurring series created",
		slog.String("series_id", series.ID.String()),
		slog.String("user_id", series.UserID),
		slog.Time("dtstart", series.DTStart),
	)

	return &schedulev1.CreateRecurringSeriesResponse{Series: toProtoRecurringSeries(series)}, nil
}

func (s *AppointmentsServer) ListOccurrences(ctx context.Context, req *schedulev1.ListOccurrencesRequest) (*schedulev1.ListOccurrencesResponse, error) {
	log := s.log.With(slog.String("rpc", "ListOccurrences"))

	if req == nil {
		log.Warn("invalid request", slog.String("reason", "nil_request"))
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.WindowStart == nil || req.WindowEnd == nil {
		log.Warn("invalid request", slog.String("reason", "missing_window"), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.InvalidArgument, "window_start and window_end are required")
	}

	occs, err := s.svc.ListOccurrences(ctx, req.UserId, req.WindowStart.AsTime(), req.WindowEnd.AsTime())
	if err != nil {
		var vErr *appointments.ValidationError
		if errors.As(err, &vErr) {
			log.Warn("invalid request", slog.Any("err", err), slog.String("user_id", req.UserId))
			return nil, status.Error(codes.InvalidArgument, vErr.Error())
		}
		log.Error("occurrences list failed", slog.Any("err", err), slog.String("user_id", req.UserId))
		return nil, status.Error(codes.Internal, "internal error")
	}

	out := make([]*schedulev1.Occurrence, 0, len(occs))
	for _, o := range occs {
		out = append(out, toProtoOccurrence(o))
	}

	log.Debug(
		"occurrences listed",
		slog.String("user_id", req.UserId),
		slog.Int("count", len(out)),
		slog.Time("window_start", req.WindowStart.AsTime()),
		slog.Time("window_end", req.WindowEnd.AsTime()),
	)

	return &schedulev1.ListOccurrencesResponse{Occurrences: out}, nil
}

func toProtoAppointment(a domain.Appointment) *schedulev1.Appointment {
	return &schedulev1.Appointment{
		Id:        a.ID.String(),
		UserId:    a.UserID,
		Title:     a.Title,
		Notes:     a.Notes,
		StartTime: timestamppb.New(a.StartTime),
		EndTime:   timestamppb.New(a.EndTime),
		CreatedAt: timestamppb.New(a.CreatedAt),
		UpdatedAt: timestamppb.New(a.UpdatedAt),
	}
}

func toProtoRecurringSeries(s domain.RecurringSeries) *schedulev1.RecurringSeries {
	duration := time.Duration(s.DurationSeconds) * time.Second

	return &schedulev1.RecurringSeries{
		Id:        s.ID.String(),
		UserId:    s.UserID,
		Title:     s.Title,
		Notes:     s.Notes,
		StartTime: timestamppb.New(s.DTStart),
		EndTime:   timestamppb.New(s.DTStart.Add(duration)),
		Weekly:    toProtoWeeklyRecurrence(s),
		CreatedAt: timestamppb.New(s.CreatedAt),
		UpdatedAt: timestamppb.New(s.UpdatedAt),
	}
}

func toProtoWeeklyRecurrence(s domain.RecurringSeries) *schedulev1.WeeklyRecurrence {
	weekdays := make([]schedulev1.Weekday, 0, len(s.ByWeekday))
	for _, wd := range s.ByWeekday {
		if wd < 1 || wd > 7 {
			continue
		}
		weekdays = append(weekdays, schedulev1.Weekday(wd))
	}

	var until *timestamppb.Timestamp
	if s.Until != nil {
		until = timestamppb.New(s.Until.UTC())
	}

	var count uint32
	if s.Count != nil && *s.Count > 0 {
		count = uint32(*s.Count)
	}

	return &schedulev1.WeeklyRecurrence{
		Interval: uint32(s.Interval),
		Weekdays: weekdays,
		Until:    until,
		Count:    count,
		TimeZone: s.Timezone,
	}
}

func toProtoOccurrence(o domain.RecurringOccurrence) *schedulev1.Occurrence {
	return &schedulev1.Occurrence{
		SeriesId:     o.SeriesID.String(),
		OccurrenceId: o.ID,
		UserId:       o.UserID,
		Title:        o.Title,
		Notes:        o.Notes,
		StartTime:    timestamppb.New(o.StartTime),
		EndTime:      timestamppb.New(o.EndTime),
	}
}
