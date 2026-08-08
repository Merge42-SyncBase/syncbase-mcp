package config

import "testing"

func TestFloat64RejectsOutOfRangeValue(t *testing.T) {
	t.Setenv("SYNCBASE_TEST_SCORE", "1.1")
	if _, err := Float64("SYNCBASE_TEST_SCORE", 0.62); err == nil {
		t.Fatal("Float64 succeeded, want range error")
	}
}
