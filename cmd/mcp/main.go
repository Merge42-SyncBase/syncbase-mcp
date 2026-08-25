package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Merge42-SyncBase/syncbase-mcp/internal/config"
	"github.com/Merge42-SyncBase/syncbase-mcp/internal/mcpserver"
	"github.com/Merge42-SyncBase/syncbase-was/searchruntime"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("MCP server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	databaseURL, err := config.Required("SYNCBASE_DATABASE_URL")
	if err != nil {
		return err
	}
	modelPath, err := config.Required("SYNCBASE_MODEL_PATH")
	if err != nil {
		return err
	}
	tokenizerPath, err := config.Required("SYNCBASE_TOKENIZER_PATH")
	if err != nil {
		return err
	}
	runtimeLibrary, err := config.Required("SYNCBASE_ORT_LIBRARY_PATH")
	if err != nil {
		return err
	}
	tokenDigest, err := config.Required("SYNCBASE_MCP_TOKEN_SHA256")
	if err != nil {
		return err
	}
	publicBaseURL, err := config.Required("SYNCBASE_PUBLIC_BASE_URL")
	if err != nil {
		return err
	}
	originalRoot, err := config.Required("SYNCBASE_ORIGINAL_ROOT")
	if err != nil {
		return err
	}
	minimumScore, err := config.Float64("SYNCBASE_MINIMUM_SCORE", 0.62)
	if err != nil {
		return err
	}
	searchRuntime, err := searchruntime.Open(ctx, searchruntime.Config{
		DatabaseURL:        databaseURL,
		ModelPath:          modelPath,
		TokenizerPath:      tokenizerPath,
		RuntimeLibraryPath: runtimeLibrary,
		PublicBaseURL:      publicBaseURL,
		OriginalRoot:       originalRoot,
		MinimumScore:       minimumScore,
	})
	if err != nil {
		return err
	}
	defer func() { _ = searchRuntime.Close() }()

	mcpHandler, err := mcpserver.New(mcpserver.Config{
		TokenSHA256:    tokenDigest,
		AllowedHosts:   config.CSV("SYNCBASE_MCP_ALLOWED_HOSTS", []string{"localhost", "127.0.0.1"}),
		AllowedOrigins: config.CSV("SYNCBASE_MCP_ALLOWED_ORIGINS", nil),
	}, searchRuntime)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("GET /healthz", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"status":"ok","runtime":"go"}`))
	})
	mux.HandleFunc("GET /readyz", func(response http.ResponseWriter, request *http.Request) {
		readyContext, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if err := searchRuntime.Ready(readyContext); err != nil {
			writeReadiness(response, http.StatusServiceUnavailable, "not_ready")
			return
		}
		writeReadiness(response, http.StatusOK, "ready")
	})
	server := &http.Server{
		Addr:              config.Value("SYNCBASE_MCP_ADDR", ":8081"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	slog.Info("MCP server ready", "address", server.Addr)
	err = server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-shutdownDone
	return ctx.Err()
}

func writeReadiness(response http.ResponseWriter, status int, value string) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_, _ = response.Write([]byte(`{"status":"` + value + `"}`))
}
