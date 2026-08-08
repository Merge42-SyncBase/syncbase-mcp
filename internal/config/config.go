// Package config parses strict MCP process configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Required returns a non-empty environment value or an error.
func Required(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

// Value returns an environment value or fallback when empty.
func Value(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

// Float64 parses a finite positive environment value or returns fallback.
func Float64(name string, fallback float64) (float64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 || parsed > 1 {
		return 0, fmt.Errorf("parse %s: value must be between 0 and 1", name)
	}
	return parsed, nil
}

// CSV returns trimmed non-empty values from a comma-separated environment value.
func CSV(name string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}
