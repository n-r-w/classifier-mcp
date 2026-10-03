package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// configSuite checks startup configuration with temporary process-environment values.
type configSuite struct {
	// Suite supplies assertion and lifecycle state for the isolated entry-point cases.
	suite.Suite
}

// TestConfig runs serial environment cases and restores the original settings after the suite.
func TestConfig(t *testing.T) {
	t.Setenv("SYSTEM_ONE_ENDPOINT", "")
	suite.Run(t, new(configSuite))
}

// SetupTest isolates every connection setting before a case selects its own overrides.
func (s *configSuite) SetupTest() {
	s.T().Setenv("SYSTEM_ONE_ENDPOINT", "http://localhost:1234/systemone")
	s.T().Setenv("SYSTEM_ONE_MODEL", "fixture-model")
	s.T().Setenv("SYSTEM_ONE_API_KEY", "")
	s.T().Setenv("SYSTEM_ONE_HTTP_TIMEOUT", "60s")
}

// TestRequiredSettings checks explicit model and endpoint requirements and safe URL validation.
func (s *configSuite) TestRequiredSettings() {
	s.T().Setenv("SYSTEM_ONE_ENDPOINT", "")
	s.T().Setenv("SYSTEM_ONE_MODEL", "")
	_, err := Load()
	s.Require().Error(err)
	s.Contains(err.Error(), "SYSTEM_ONE_ENDPOINT")
	s.T().Setenv("SYSTEM_ONE_ENDPOINT", "http://localhost:1234/systemone")
	_, err = Load()
	s.Require().Error(err)
	s.Contains(err.Error(), "SYSTEM_ONE_MODEL")
	s.T().Setenv("SYSTEM_ONE_MODEL", "chosen-alias")
	s.T().Setenv("SYSTEM_ONE_API_KEY", "secret")
	cfg, err := Load()
	s.Require().NoError(err)
	s.Equal("chosen-alias", cfg.Model)
	s.Equal("http://localhost:1234/systemone", cfg.Endpoint)
	s.Equal("secret", cfg.APIKey)
	s.T().Setenv("SYSTEM_ONE_ENDPOINT", "file:///secret")
	_, err = Load()
	s.Require().Error(err)
	s.NotContains(err.Error(), "secret")
}

// TestHTTPTimeout checks the default, duration override, and rejected operational settings.
func (s *configSuite) TestHTTPTimeout() {
	s.T().Setenv("SYSTEM_ONE_ENDPOINT", "http://localhost:1234/systemone")
	s.T().Setenv("SYSTEM_ONE_MODEL", "chosen")
	s.T().Setenv("SYSTEM_ONE_HTTP_TIMEOUT", "")
	cfg, err := Load()
	s.Require().NoError(err)
	s.Equal(time.Minute, cfg.HTTPTimeout)
	s.T().Setenv("SYSTEM_ONE_HTTP_TIMEOUT", "250ms")
	cfg, err = Load()
	s.Require().NoError(err)
	s.Equal(250*time.Millisecond, cfg.HTTPTimeout)
	for _, value := range []string{"0s", "-1s", "invalid"} {
		s.T().Setenv("SYSTEM_ONE_HTTP_TIMEOUT", value)
		_, err = Load()
		s.Require().Error(err)
		s.Contains(err.Error(), "SYSTEM_ONE_HTTP_TIMEOUT")
	}
}
