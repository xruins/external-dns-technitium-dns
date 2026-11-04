package config

import (
	"reflect"
	"testing"
)

func setRequired(t *testing.T) {
	t.Setenv("TECHNITIUM_URL", "http://localhost:5380")
	t.Setenv("TECHNITIUM_USERNAME", "u")
	t.Setenv("TECHNITIUM_PASSWORD", "p")
}

func TestLoad_zones(t *testing.T) {
	setRequired(t)
	t.Setenv("ZONE", "ignored.example")
	t.Setenv("ZONES", "Example.com.; a.example.com ;")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"example.com", "a.example.com"}; !reflect.DeepEqual(cfg.Zones, want) {
		t.Errorf("Zones = %v, want %v", cfg.Zones, want)
	}
}

func TestLoad_zoneFallback(t *testing.T) {
	setRequired(t)
	t.Setenv("ZONE", "example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{"example.com"}; !reflect.DeepEqual(cfg.Zones, want) {
		t.Errorf("Zones = %v, want %v", cfg.Zones, want)
	}
}

func TestLoad_zoneRequired(t *testing.T) {
	setRequired(t)
	t.Setenv("ZONE", "")
	t.Setenv("ZONES", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when no zone is configured")
	}
}
