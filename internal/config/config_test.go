package config

import (
	"os"
	"testing"
	"time"
)

func TestGetEnv(t *testing.T) {
	key := "TEST_ENV_VAR"
	defaultValue := "default"

	// Test default value
	os.Unsetenv(key)
	if got := getEnv(key, defaultValue); got != defaultValue {
		t.Errorf("getEnv() = %v, want %v", got, defaultValue)
	}

	// Test set value
	want := "custom"
	os.Setenv(key, want)
	defer os.Unsetenv(key)
	if got := getEnv(key, defaultValue); got != want {
		t.Errorf("getEnv() = %v, want %v", got, want)
	}
}

func TestGetEnvInt(t *testing.T) {
	key := "TEST_ENV_INT"
	defaultVal := 123

	// Test default value
	os.Unsetenv(key)
	if got := getEnvInt(key, defaultVal); got != defaultVal {
		t.Errorf("getEnvInt() = %v, want %v", got, defaultVal)
	}

	// Test valid int
	os.Setenv(key, "456")
	if got := getEnvInt(key, defaultVal); got != 456 {
		t.Errorf("getEnvInt() = %v, want %v", got, 456)
	}

	// Test invalid int
	os.Setenv(key, "invalid")
	if got := getEnvInt(key, defaultVal); got != defaultVal {
		t.Errorf("getEnvInt() = %v, want %v", got, defaultVal)
	}
	os.Unsetenv(key)
}

func TestGetEnvDuration(t *testing.T) {
	key := "TEST_ENV_DURATION"
	defaultVal := 5 * time.Minute

	// Test default value
	os.Unsetenv(key)
	if got := getEnvDuration(key, defaultVal); got != defaultVal {
		t.Errorf("getEnvDuration() = %v, want %v", got, defaultVal)
	}

	// Test valid duration
	os.Setenv(key, "10m")
	if got := getEnvDuration(key, defaultVal); got != 10*time.Minute {
		t.Errorf("getEnvDuration() = %v, want %v", got, 10*time.Minute)
	}

	// Test invalid duration
	os.Setenv(key, "invalid")
	if got := getEnvDuration(key, defaultVal); got != defaultVal {
		t.Errorf("getEnvDuration() = %v, want %v", got, defaultVal)
	}
	os.Unsetenv(key)
}

func TestGetEnvBool(t *testing.T) {
	key := "TEST_ENV_BOOL"
	defaultVal := false

	// Test default value
	os.Unsetenv(key)
	if got := getEnvBool(key, defaultVal); got != defaultVal {
		t.Errorf("getEnvBool() = %v, want %v", got, defaultVal)
	}

	// Test valid bool
	os.Setenv(key, "true")
	if got := getEnvBool(key, defaultVal); got != true {
		t.Errorf("getEnvBool() = %v, want %v", got, true)
	}

	// Test invalid bool
	os.Setenv(key, "invalid")
	if got := getEnvBool(key, defaultVal); got != defaultVal {
		t.Errorf("getEnvBool() = %v, want %v", got, defaultVal)
	}
	os.Unsetenv(key)
}

func TestLoad(t *testing.T) {
	// Set some env vars to override defaults
	os.Setenv("PORT", "9999")
	os.Setenv("DB_PORT", "1111")
	defer os.Unsetenv("PORT")
	defer os.Unsetenv("DB_PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != "9999" {
		t.Errorf("Expected Port 9999, got %s", cfg.Server.Port)
	}
	if cfg.DB.Port != 1111 {
		t.Errorf("Expected DB Port 1111, got %d", cfg.DB.Port)
	}
}
