package config

import "testing"

func TestCountryConfigurationFallbacksAndAliases(t *testing.T) {
	if _, err := LoadCountries(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, currency string }{
		{" Spain ", "EUR"}, {"JAPAN", "JPY"}, {"UK", "GBP"}, {"usa", "USD"}, {"unknown country", ""},
	} {
		if got := Country(tc.name).Currency; got != tc.currency {
			t.Errorf("%q currency=%q, want %q", tc.name, got, tc.currency)
		}
	}
	if Country("Japan").MenuTerms != "メニュー 料金" {
		t.Fatal("non-English search vocabulary was corrupted")
	}
}

func TestModelCallCapConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://unused-local-test")
	t.Setenv("APP_ENV", "local")
	t.Setenv("MODEL_PROVIDER", "gemini")
	t.Setenv("PORT", "8080")
	for _, tc := range []struct {
		value string
		want  int
	}{{"", 9}, {"2", 2}, {"100", 20}, {"-1", 9}, {"invalid", 9}} {
		t.Setenv("MAX_MODEL_CALLS_PER_RUN", tc.value)
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.MaxModelCalls != tc.want {
			t.Errorf("%q cap=%d, want %d", tc.value, cfg.MaxModelCalls, tc.want)
		}
	}
}
