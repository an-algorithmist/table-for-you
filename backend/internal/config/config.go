package config

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds validated process settings loaded once at startup.
type Config struct {
	MaxModelCalls                                                        int
	GroundedModel                                                        string
	GroundedOwnerQuota, GroundedGlobalQuota, GroundedMaxOutput           int
	GroundedEnabled                                                      bool
	DatabaseURL, GeminiKey, TavilyKey, Model, Provider, Port, AccessCode string
	Local                                                                bool
	OwnerQuota, GlobalQuota                                              int
	RunTimeout                                                           time.Duration
}

// LoadEnv loads an optional private env file without overriding process variables.
func LoadEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(strings.TrimPrefix(line, "export "), "=")
		if !ok {
			return errors.New("invalid .env entry")
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if _, exists := os.LookupEnv(k); !exists {
			if err := os.Setenv(k, v); err != nil {
				return err
			}
		}
	}
	return s.Err()
}

// Load validates required settings and applies bounded assignment defaults.
func Load() (Config, error) {
	if _, err := LoadCountries(); err != nil {
		return Config{}, err
	}
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), GeminiKey: os.Getenv("GEMINI_API_KEY"), TavilyKey: os.Getenv("TAVILY_API_KEY"), Model: env("MODEL_NAME", "gemini-3.1-flash-lite"), Provider: env("MODEL_PROVIDER", "gemini"), Port: env("PORT", "8080"), AccessCode: os.Getenv("REVIEWER_ACCESS_CODE"), Local: env("APP_ENV", "local") == "local", OwnerQuota: number("MAX_DAILY_RUNS_PER_OWNER", 20), GlobalQuota: number("MAX_DAILY_RUNS_GLOBAL", 100), RunTimeout: 180 * time.Second}
	c.MaxModelCalls = min(number("MAX_MODEL_CALLS_PER_RUN", 9), 20)
	c.GroundedModel = env("GROUNDED_MODEL_NAME", "gemini-3.8-flash")
	c.GroundedEnabled = env("ENABLE_GOOGLE_GROUNDED", "true") == "true"
	c.GroundedOwnerQuota = number("MAX_DAILY_GROUNDED_PER_OWNER", 3)
	c.GroundedGlobalQuota = number("MAX_DAILY_GROUNDED_GLOBAL", 10)
	c.GroundedMaxOutput = min(number("GROUNDED_MAX_OUTPUT_TOKENS", 8192), 8192)
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required; see .env.example")
	}
	if !c.Local && c.AccessCode == "" {
		return c, errors.New("REVIEWER_ACCESS_CODE is required outside local mode")
	}
	if c.Provider != "gemini" {
		return c, errors.New("unsupported MODEL_PROVIDER; only gemini is implemented")
	}
	if _, err := strconv.Atoi(c.Port); err != nil {
		return c, errors.New("PORT must be numeric")
	}
	return c, nil
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func number(k string, d int) int {
	n, e := strconv.Atoi(os.Getenv(k))
	if e == nil && n > 0 {
		return n
	}
	return d
}
