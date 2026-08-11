package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var ErrDatabaseURLRequired = errors.New("DATABASE_URL is required")

type Database struct {
	URL             string
	MaxConnections  int32
	MinConnections  int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
}

func LoadDatabase() (Database, error) {
	// Environment variables already set by the operating system take precedence.
	_ = godotenv.Load()
	return databaseFromEnvironment()
}

func databaseFromEnvironment() (Database, error) {
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		return Database{}, ErrDatabaseURLRequired
	}

	maxConnections, err := positiveInt("DB_MAX_CONNECTIONS", 10)
	if err != nil {
		return Database{}, err
	}
	minConnections, err := nonNegativeInt("DB_MIN_CONNECTIONS", 2)
	if err != nil {
		return Database{}, err
	}
	if minConnections > maxConnections {
		return Database{}, errors.New("DB_MIN_CONNECTIONS cannot exceed DB_MAX_CONNECTIONS")
	}

	maxLifetime, err := duration("DB_MAX_CONN_LIFETIME", 30*time.Minute)
	if err != nil {
		return Database{}, err
	}
	connectTimeout, err := duration("DB_CONNECT_TIMEOUT", 5*time.Second)
	if err != nil {
		return Database{}, err
	}

	return Database{
		URL:             url,
		MaxConnections:  int32(maxConnections),
		MinConnections:  int32(minConnections),
		MaxConnLifetime: maxLifetime,
		ConnectTimeout:  connectTimeout,
	}, nil
}

func positiveInt(name string, fallback int) (int, error) {
	value, err := integer(name, fallback)
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return value, nil
}

func nonNegativeInt(name string, fallback int) (int, error) {
	value, err := integer(name, fallback)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("%s cannot be negative", name)
	}
	return value, nil
}

func integer(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	return value, nil
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", name, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return value, nil
}
