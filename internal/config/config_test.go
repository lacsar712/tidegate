package config_test

import (
	"testing"
	"time"

	"github.com/lacsar712/tidegate/internal/config"
)

func TestDefaultValidate(t *testing.T) {
	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRandomSecret(t *testing.T) {
	s, err := config.NewRandomSecret(16)
	if err != nil || len(s) < 16 {
		t.Fatalf("secret=%q err=%v", s, err)
	}
}

func TestInvalidSmoothN(t *testing.T) {
	cfg := config.Default()
	cfg.SmoothN = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestTicketTTLPositive(t *testing.T) {
	cfg := config.Default()
	cfg.TicketTTL = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected ttl validation error")
	}
	cfg.TicketTTL = 15 * time.Second
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
