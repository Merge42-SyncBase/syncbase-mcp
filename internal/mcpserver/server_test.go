package mcpserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	versionID := uuid.New()
	searcher := &groundedFixtureSearcher{status: groundingSupported, hits: []searchruntime.Hit{{
		Rank:            1,
		Score:           0.91,
		DocumentID:      documentID.String(),
		DocumentName:    "보안 정책",
		VersionID:       versionID.String(),
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
				GroundingStatus string      `json:"grounding_status"`
				GroundingReason *string     `json:"grounding_reason"`
				Results         []searchHit `json:"results"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
	if len(payload.Result.StructuredContent.Results) != 1 {
		t.Fatalf("results = %+v; body=%s", payload.Result.StructuredContent.Results, response.Body.String())
	}
	if payload.Result.StructuredContent.GroundingStatus != "SUPPORTED" ||
		payload.Result.StructuredContent.GroundingReason != nil {
		t.Fatalf("grounding = %+v; body=%s", payload.Result.StructuredContent, response.Body.String())
	}
	got := payload.Result.StructuredContent.Results[0]
	if got.VersionID != versionID.String() || got.DocumentVersion != 2 || got.PageNumber != 3 ||
		got.SourceURL != searcher.hits[0].SourceURL {
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

func TestSearchDocumentsFailsClosedForLegacyHitsWithoutGroundingMetadata(t *testing.T) {
	t.Parallel()

	token := "sb_mcp_v1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	digest := sha256.Sum256([]byte(token))
	searcher := &fixtureSearcher{hits: []searchruntime.Hit{{Rank: 1, Score: 0.91, Snippet: "unverified legacy hit"}}}
	handler, err := New(Config{
		TokenSHA256:  hex.EncodeToString(digest[:]),
		AllowedHosts: []string{"example.test"},
	}, searcher)
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
			IsError           bool `json:"isError"`
			StructuredContent struct {
				GroundingStatus string      `json:"grounding_status"`
				GroundingReason string      `json:"grounding_reason"`
				Results         []searchHit `json:"results"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
	if payload.Result.IsError || payload.Result.StructuredContent.GroundingStatus != groundingInsufficientEvidence ||
		payload.Result.StructuredContent.GroundingReason != groundingSourceUnavailable ||
		payload.Result.StructuredContent.Results == nil || len(payload.Result.StructuredContent.Results) != 0 {
		t.Fatalf("grounding output = %+v; body=%s", payload.Result, response.Body.String())
	}
	if searcher.query != "query" || searcher.limit != 5 {
		t.Fatalf("legacy search call: query=%q limit=%d", searcher.query, searcher.limit)
	}
}

func TestSearchDocumentsReturnsExplicitEmptyEvidenceForEverySafetyFailure(t *testing.T) {
	tests := []struct {
		name   string
		status string
		reason string
		err    error
	}{
		{name: "no hits", status: "INSUFFICIENT_EVIDENCE", reason: "NO_HITS_ABOVE_POLICY"},
		{name: "inactive only", status: "INSUFFICIENT_EVIDENCE", reason: "ONLY_INACTIVE_VERSION_MATCHED"},
		{name: "source unavailable", err: searchruntime.ErrTemporarilyUnavailable, reason: "SOURCE_UNAVAILABLE"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := "sb_mcp_v1_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
			digest := sha256.Sum256([]byte(token))
			searcher := &groundedFixtureSearcher{status: test.status, reason: test.reason, failure: test.err}
			handler, err := New(Config{
				TokenSHA256: hex.EncodeToString(digest[:]), AllowedHosts: []string{"example.test"},
			}, searcher)
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
					IsError           bool `json:"isError"`
					StructuredContent struct {
						GroundingStatus string      `json:"grounding_status"`
						GroundingReason string      `json:"grounding_reason"`
						Results         []searchHit `json:"results"`
					} `json:"structuredContent"`
				} `json:"result"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
			}
			if payload.Result.IsError || payload.Result.StructuredContent.GroundingStatus != "INSUFFICIENT_EVIDENCE" ||
				payload.Result.StructuredContent.GroundingReason != test.reason ||
				payload.Result.StructuredContent.Results == nil || len(payload.Result.StructuredContent.Results) != 0 {
				t.Fatalf("grounding response = %+v; body=%s", payload.Result, response.Body.String())
			}
		})
	}
}

func TestSearchDocumentsTurnsLegacyRetryableDependencyFailureIntoEmptyEvidence(t *testing.T) {
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
			IsError           bool `json:"isError"`
			StructuredContent struct {
				GroundingStatus string      `json:"grounding_status"`
				GroundingReason string      `json:"grounding_reason"`
				Results         []searchHit `json:"results"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
	if payload.Result.IsError || payload.Result.StructuredContent.GroundingStatus != groundingInsufficientEvidence ||
		payload.Result.StructuredContent.GroundingReason != groundingSourceUnavailable ||
		payload.Result.StructuredContent.Results == nil || len(payload.Result.StructuredContent.Results) != 0 {
		t.Fatalf("grounding output = %+v; body=%s", payload.Result, response.Body.String())
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

type groundedFixtureSearcher struct {
	status  string
	reason  string
	failure error
	hits    []searchruntime.Hit
	query   string
	limit   int
}

func (s *groundedFixtureSearcher) Documents(context.Context, string, int) ([]searchruntime.Hit, error) {
	return nil, errors.New("legacy search path must not be used")
}

func (s *groundedFixtureSearcher) GroundedDocuments(
	_ context.Context,
	query string,
	limit int,
) ([]searchruntime.Hit, string, string, error) {
	s.query = query
	s.limit = limit
	return s.hits, s.status, s.reason, s.failure
}
