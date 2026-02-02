package config

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	GRPCHost           string
	GRPCPort           int
	DatabaseURL        string
	ShutdownTimeout    time.Duration
	LogLevel           string
	GRPCRequestTimeout time.Duration
	DBMaxOpenConns     int
	DBMaxIdleConns     int
	DBConnMaxLifetime  time.Duration
	DBConnMaxIdleTime  time.Duration
}

func Load() (Config, error) {
	v := viper.New()
	v.SetEnvPrefix("SCHEDULA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("grpc.host", "0.0.0.0")
	v.SetDefault("grpc.port", 50051)
	v.SetDefault("grpc.addr", "")
	v.SetDefault("grpc.request_timeout", "10s")
	v.SetDefault("database.url", "postgres://schedula:schedula@127.0.0.1:5433/schedula?sslmode=disable")
	v.SetDefault("database.max_open_conns", 20)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", "30m")
	v.SetDefault("database.conn_max_idle_time", "5m")
	v.SetDefault("shutdown.timeout", "10s")
	v.SetDefault("log.level", "info")

	_ = v.BindEnv("grpc.host", "SCHEDULA_GRPC_HOST", "GRPC_HOST")
	_ = v.BindEnv("grpc.port", "SCHEDULA_GRPC_PORT", "GRPC_PORT", "PORT")
	_ = v.BindEnv("grpc.addr", "SCHEDULA_GRPC_ADDR", "GRPC_ADDR")
	_ = v.BindEnv("grpc.request_timeout", "SCHEDULA_GRPC_REQUEST_TIMEOUT")
	_ = v.BindEnv("database.url", "SCHEDULA_DATABASE_URL", "DATABASE_URL")
	_ = v.BindEnv("database.max_open_conns", "SCHEDULA_DATABASE_MAX_OPEN_CONNS")
	_ = v.BindEnv("database.max_idle_conns", "SCHEDULA_DATABASE_MAX_IDLE_CONNS")
	_ = v.BindEnv("database.conn_max_lifetime", "SCHEDULA_DATABASE_CONN_MAX_LIFETIME")
	_ = v.BindEnv("database.conn_max_idle_time", "SCHEDULA_DATABASE_CONN_MAX_IDLE_TIME")
	_ = v.BindEnv("shutdown.timeout", "SCHEDULA_SHUTDOWN_TIMEOUT", "SHUTDOWN_TIMEOUT")
	_ = v.BindEnv("log.level", "SCHEDULA_LOG_LEVEL", "LOG_LEVEL")

	timeout, err := time.ParseDuration(v.GetString("shutdown.timeout"))
	if err != nil {
		return Config{}, err
	}

	grpcTimeout, err := time.ParseDuration(v.GetString("grpc.request_timeout"))
	if err != nil {
		return Config{}, err
	}

	connMaxLifetime, err := time.ParseDuration(v.GetString("database.conn_max_lifetime"))
	if err != nil {
		return Config{}, err
	}
	connMaxIdleTime, err := time.ParseDuration(v.GetString("database.conn_max_idle_time"))
	if err != nil {
		return Config{}, err
	}

	if addr := strings.TrimSpace(v.GetString("grpc.addr")); addr != "" {
		host, portStr, err := net.SplitHostPort(addr)
		if err == nil {
			if host != "" {
				v.Set("grpc.host", host)
			}
			if port, err := strconv.Atoi(portStr); err == nil {
				v.Set("grpc.port", port)
			}
		}
	}

	return Config{
		GRPCHost:           strings.TrimSpace(v.GetString("grpc.host")),
		GRPCPort:           v.GetInt("grpc.port"),
		DatabaseURL:        v.GetString("database.url"),
		ShutdownTimeout:    timeout,
		LogLevel:           v.GetString("log.level"),
		GRPCRequestTimeout: grpcTimeout,
		DBMaxOpenConns:     v.GetInt("database.max_open_conns"),
		DBMaxIdleConns:     v.GetInt("database.max_idle_conns"),
		DBConnMaxLifetime:  connMaxLifetime,
		DBConnMaxIdleTime:  connMaxIdleTime,
	}, nil
}
