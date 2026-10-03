// Package config loads and validates immutable startup settings.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds System One connection settings.
type Config struct {
	// Endpoint is the full evaluation URL; startup requires an explicit HTTP(S) value.
	Endpoint string `env:"SYSTEM_ONE_ENDPOINT,required"`
	// Model is passed unchanged to the endpoint, including provider-specific aliases.
	Model string `env:"SYSTEM_ONE_MODEL,required"`
	// APIKey is the bearer credential; an empty value omits Authorization.
	APIKey string `env:"SYSTEM_ONE_API_KEY"`
	// HTTPTimeout bounds each complete HTTP request, including response-body reads.
	HTTPTimeout time.Duration `env:"SYSTEM_ONE_HTTP_TIMEOUT" envDefault:"60s"`
	// MaxParallelism bounds active model HTTP requests across all calls in the process.
	MaxParallelism int `env:"SYSTEM_ONE_MAX_PARALLELISM" envDefault:"4"`
	// MaxAttempts includes the first HTTP attempt and explicit overload retries.
	MaxAttempts int `env:"SYSTEM_ONE_MAX_ATTEMPTS" envDefault:"3"`
	// RetryDelay is used after overload when the endpoint supplies no valid Retry-After.
	RetryDelay time.Duration `env:"SYSTEM_ONE_RETRY_DELAY" envDefault:"1s"`
}

// Load reads and validates process environment settings.
func Load() (Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		if parseError, ok := errors.AsType[env.ParseError](err); ok {
			settings := map[string]string{
				"HTTPTimeout":    "SYSTEM_ONE_HTTP_TIMEOUT",
				"MaxParallelism": "SYSTEM_ONE_MAX_PARALLELISM",
				"MaxAttempts":    "SYSTEM_ONE_MAX_ATTEMPTS",
				"RetryDelay":     "SYSTEM_ONE_RETRY_DELAY",
			}
			if setting, present := settings[parseError.Name]; present {
				return Config{}, fmt.Errorf("%s: %w", setting, parseError.Err)
			}
		}
		return Config{}, fmt.Errorf("load configuration: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// validate checks connection and execution controls before the application accepts calls.
func (c Config) validate() error {
	if strings.TrimSpace(c.Endpoint) == "" {
		return errors.New("SYSTEM_ONE_ENDPOINT is required")
	}
	if strings.TrimSpace(c.Model) == "" {
		return errors.New("SYSTEM_ONE_MODEL is required")
	}
	endpoint, err := url.Parse(c.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") ||
		endpoint.User != nil ||
		endpoint.Fragment != "" {
		return errors.New(
			"SYSTEM_ONE_ENDPOINT must be an HTTP(S) URL without credentials or fragment",
		)
	}
	if strings.ContainsAny(c.APIKey, "\r\n") {
		return errors.New("SYSTEM_ONE_API_KEY must not contain a line break")
	}
	if c.HTTPTimeout <= 0 {
		return errors.New("SYSTEM_ONE_HTTP_TIMEOUT must be greater than zero")
	}
	if c.MaxParallelism <= 0 {
		return errors.New("SYSTEM_ONE_MAX_PARALLELISM must be greater than zero")
	}
	if c.MaxAttempts <= 0 {
		return errors.New("SYSTEM_ONE_MAX_ATTEMPTS must be greater than zero")
	}
	if c.RetryDelay < 0 {
		return errors.New("SYSTEM_ONE_RETRY_DELAY must be non-negative")
	}
	return nil
}
