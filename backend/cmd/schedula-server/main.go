package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"schedula/backend/internal/config"
	schedulev1 "schedula/backend/internal/gen/proto/schedula/v1"
	"schedula/backend/internal/service/appointments"
	"schedula/backend/internal/store/postgres"
	grpcTransport "schedula/backend/internal/transport/grpc"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).With(
		slog.String("service", "schedula-server"),
	)
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		log.Error("config load failed", slog.Any("err", err))
		os.Exit(1)
	}

	log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLogLevel(cfg.LogLevel)})).With(
		slog.String("service", "schedula-server"),
	)
	slog.SetDefault(log)

	log.Info("starting", slog.String("grpc_addr", cfg.GRPCAddr), slog.String("log_level", cfg.LogLevel))

	log.Info("connecting to database", databaseLogArgs(cfg.DatabaseURL)...)
	db, err := postgres.Open(cfg.DatabaseURL, postgres.PoolConfig{
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
		ConnMaxIdleTime: cfg.DBConnMaxIdleTime,
	})
	if err != nil {
		args := append([]any{slog.Any("err", err)}, databaseLogArgs(cfg.DatabaseURL)...)
		log.Error("database connection failed", args...)
		os.Exit(1)
	}
	defer func() {
		if err := postgres.Close(db); err != nil {
			log.Warn("database close failed", slog.Any("err", err))
		}
	}()

	repo := postgres.NewAppointmentRepo(db)
	svc := appointments.NewService(repo)

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(defaultRequestTimeoutInterceptor(cfg.GRPCRequestTimeout)),
	)
	schedulev1.RegisterAppointmentsServiceServer(grpcServer, grpcTransport.NewAppointmentsServer(svc, log))

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Error("grpc listen failed", slog.Any("err", err), slog.String("grpc_addr", cfg.GRPCAddr))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- grpcServer.Serve(lis)
	}()

	log.Info("grpc server started", slog.String("grpc_addr", cfg.GRPCAddr))

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
		shutdown(log, grpcServer, cfg.ShutdownTimeout)
	case err := <-errCh:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Error("grpc server stopped with error", slog.Any("err", err))
			os.Exit(1)
		}
	}
}

func defaultRequestTimeoutInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := ctx.Deadline(); ok {
			return handler(ctx, req)
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		return handler(ctx, req)
	}
}

func shutdown(log *slog.Logger, s *grpc.Server, timeout time.Duration) {
	log.Info("shutting down grpc server", slog.Duration("timeout", timeout))

	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		log.Info("grpc server stopped")
	case <-timer.C:
		log.Warn("grpc graceful shutdown timed out; forcing stop")
		s.Stop()
	}
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func databaseLogArgs(databaseURL string) []any {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return []any{slog.String("db_url", "invalid")}
	}
	name := strings.TrimPrefix(u.Path, "/")
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "default"
	}
	if host == "" {
		host = "unknown"
	}
	if name == "" {
		name = "unknown"
	}
	return []any{
		slog.String("db_host", host),
		slog.String("db_port", port),
		slog.String("db_name", name),
	}
}
