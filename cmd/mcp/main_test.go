package main

import (
	"strings"
	"testing"
)

func TestLoadSearchRuntimeConfigRequiresOriginalRoot(t *testing.T) {
	setSearchRuntimeEnvironment(t)
	t.Setenv("SYNCBASE_ORIGINAL_ROOT", "   ")

	_, err := loadSearchRuntimeConfig()
	if err == nil || !strings.Contains(err.Error(), "SYNCBASE_ORIGINAL_ROOT is required") {
		t.Fatalf("loadSearchRuntimeConfig error = %v, want required original-root error", err)
	}
}

func TestLoadSearchRuntimeConfigPassesOriginalRootToWAS(t *testing.T) {
	setSearchRuntimeEnvironment(t)
	t.Setenv("SYNCBASE_ORIGINAL_ROOT", "  /data/originals  ")

	got, err := loadSearchRuntimeConfig()
	if err != nil {
		t.Fatalf("loadSearchRuntimeConfig: %v", err)
	}
	if got.OriginalRoot != "/data/originals" {
		t.Fatalf("OriginalRoot = %q, want /data/originals", got.OriginalRoot)
	}
}

func setSearchRuntimeEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("SYNCBASE_DATABASE_URL", "postgres://mcp:password@database.example.test/syncbase")
	t.Setenv("SYNCBASE_MODEL_PATH", "/models/model.onnx")
	t.Setenv("SYNCBASE_TOKENIZER_PATH", "/models/tokenizer.json")
	t.Setenv("SYNCBASE_ORT_LIBRARY_PATH", "/runtime/libonnxruntime.so")
	t.Setenv("SYNCBASE_PUBLIC_BASE_URL", "https://syncbase.example.test")
	t.Setenv("SYNCBASE_MINIMUM_SCORE", "0.62")
}
