package mcpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Merge42-SyncBase/syncbase-was/searchruntime"
	"github.com/google/uuid"
)

func TestSearchDocumentsUsesAuthenticatedOfficialMCPTransport(t *testing.T) {
	token := "sb_mcp_v1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	digest := sha256.Sum256([]byte(token))
	documentID := uuid.New()
	searcher := &fixtureSearcher{hits: []searchruntime.Hit{{
		Rank:            1,
		Score:           0.91,
		DocumentID:      documentID.String(),
		DocumentName:    "보안 정책",
		DocumentVersion: 2,
		PageNumber:      3,
		Snippet:         "비밀번호는 90일마다 변경합니다.",
		SourceURL:       "http://localhost:8080/sources/" + documentID.String() + "/versions/2?page=3",
	}}}
	handler, err := New(Config{
		TokenSHA256:    hex.EncodeToString(digest[:]),
		AllowedHosts:   []string{"example.test"},
		AllowedOrigins: []string{"https://app.example.test"},
	}, searcher)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_documents","arguments":{"query":"비밀번호 정책","limit":5}}}`)
	request := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Origin", "HTTPS://APP.EXAMPLE.TEST")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Result struct {
			StructuredContent struct {
				Results []searchHit `json:"results"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
	if len(payload.Result.StructuredContent.Results) != 1 {
		t.Fatalf("results = %+v; body=%s", payload.Result.StructuredContent.Results, response.Body.String())
	}
	got := payload.Result.StructuredContent.Results[0]
	if got.DocumentVersion != 2 || got.PageNumber != 3 || got.SourceURL != searcher.hits[0].SourceURL {
		t.Fatalf("result = %+v", got)
	}
	if searcher.query != "비밀번호 정책" || searcher.limit != 5 {
		t.Fatalf("search call: query=%q limit=%d", searcher.query, searcher.limit)
	}

	unauthenticated := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", bytes.NewReader(body))
	unauthenticated.Header.Set("Content-Type", "application/json")
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, unauthenticated)
	if denied.Code != http.StatusUnauthorized || denied.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("unauthenticated status=%d headers=%v body=%s", denied.Code, denied.Header(), denied.Body.String())
	}

	badOrigin := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", bytes.NewReader(body))
	badOrigin.Header.Set("Authorization", "Bearer "+token)
	badOrigin.Header.Set("Origin", "https://attacker.example")
	forbidden := httptest.NewRecorder()
	handler.ServeHTTP(forbidden, badOrigin)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("bad origin status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
}

func TestSearchDocumentsReturnsRetryableDependencyFailure(t *testing.T) {
	t.Parallel()

	token := "sb_mcp_v1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	digest := sha256.Sum256([]byte(token))
	handler, err := New(Config{
		TokenSHA256:  hex.EncodeToString(digest[:]),
		AllowedHosts: []string{"example.test"},
	}, &fixtureSearcher{failure: searchruntime.ErrTemporarilyUnavailable})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "http://example.test/mcp", bytes.NewReader([]byte(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_documents","arguments":{"query":"query","limit":5}}}`,
	)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
	if !payload.Result.IsError || len(payload.Result.Content) != 1 ||
		payload.Result.Content[0].Text != "TEMPORARILY_UNAVAILABLE" {
		t.Fatalf("tool error = %+v; body=%s", payload.Result, response.Body.String())
	}
}

type fixtureSearcher struct {
	hits    []searchruntime.Hit
	query   string
	limit   int
	failure error
}

func (s *fixtureSearcher) Documents(_ context.Context, query string, limit int) ([]searchruntime.Hit, error) {
	s.query = query
	s.limit = limit
	return s.hits, s.failure
}
