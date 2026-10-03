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
}

// Load reads and validates process environment settings.
func Load() (Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		var parseError env.ParseError
		if errors.As(err, &parseError) && parseError.Name == "HTTPTimeout" {
			return Config{}, fmt.Errorf("SYSTEM_ONE_HTTP_TIMEOUT: %w", parseError.Err)
		}
		return Config{}, fmt.Errorf("load configuration: %w", err)
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return Config{}, errors.New("SYSTEM_ONE_ENDPOINT is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return Config{}, errors.New("SYSTEM_ONE_MODEL is required")
	}
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "https" && endpoint.Scheme != "http") ||
		endpoint.User != nil ||
		endpoint.Fragment != "" {
		return Config{}, errors.New(
			"SYSTEM_ONE_ENDPOINT must be an HTTP(S) URL without credentials or fragment",
		)
	}
	if strings.ContainsAny(cfg.APIKey, "\r\n") {
		return Config{}, errors.New("SYSTEM_ONE_API_KEY must not contain a line break")
	}
	if cfg.HTTPTimeout <= 0 {
		return Config{}, errors.New("SYSTEM_ONE_HTTP_TIMEOUT must be greater than zero")
	}
	return cfg, nil
}
