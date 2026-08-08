// Package mcpserver exposes grounded document search over official MCP Streamable HTTP.
package mcpserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/Merge42-SyncBase/syncbase-was/searchruntime"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const tokenPrefix = "sb_mcp_v1_"

// Config defines MCP transport authentication and origin boundaries.
type Config struct {
	TokenSHA256    string
	AllowedHosts   []string
	AllowedOrigins []string
}

type searcher interface {
	Documents(context.Context, string, int) ([]searchruntime.Hit, error)
}

type searchInput struct {
	Query string `json:"query" jsonschema:"semantic search query"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of results"`
}

type searchOutput struct {
	Results []searchHit `json:"results"`
}

type searchHit struct {
	Rank            int     `json:"rank"`
	Score           float64 `json:"score"`
	DocumentID      string  `json:"document_id"`
	DocumentName    string  `json:"document_name"`
	DocumentVersion int     `json:"document_version"`
	PageNumber      int     `json:"page_number"`
	Snippet         string  `json:"snippet"`
	SourceURL       string  `json:"source_url"`
}

// New returns an authenticated, stateless MCP Streamable HTTP handler.
func New(config Config, searcher searcher) (http.Handler, error) {
	expectedDigest, err := decodeDigest(config.TokenSHA256)
	if err != nil {
		return nil, err
	}
	hosts := normalizedSet(config.AllowedHosts)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("at least one allowed host is required: %w", searchruntime.ErrInvalidArgument)
	}
	origins := normalizedSet(config.AllowedOrigins)
	if searcher == nil {
		return nil, searchruntime.ErrInvalidArgument
	}
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "syncbase",
		Version: "0.2.0-go",
	}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_documents",
		Title:       "Search grounded documents",
		Description: "활성 문서 버전에서 근거 페이지와 snippet을 의미 검색합니다.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"query"},
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "minLength": 1, "maxLength": 2000},
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 20, "default": 5},
			},
			"additionalProperties": false,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input searchInput) (*mcp.CallToolResult, searchOutput, error) {
		hits, err := searcher.Documents(ctx, input.Query, input.Limit)
		if err != nil {
			return nil, searchOutput{}, safeToolError(err)
		}
		output := searchOutput{Results: make([]searchHit, len(hits))}
		for index, hit := range hits {
			output.Results[index] = searchHit{
				Rank:            hit.Rank,
				Score:           hit.Score,
				DocumentID:      hit.DocumentID,
				DocumentName:    hit.DocumentName,
				DocumentVersion: hit.DocumentVersion,
				PageNumber:      hit.PageNumber,
				Snippet:         hit.Snippet,
				SourceURL:       hit.SourceURL,
			}
		}
		return nil, output, nil
	})
	transport := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)
	return securityMiddleware(expectedDigest, hosts, origins, transport), nil
}

func securityMiddleware(
	expectedDigest []byte,
	hosts map[string]struct{},
	origins map[string]struct{},
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		host := strings.ToLower(request.Host)
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			host = parsed
		}
		if _, allowed := hosts[host]; !allowed {
			writeSecurityError(response, http.StatusForbidden, "FORBIDDEN")
			return
		}
		if origin := strings.TrimSpace(request.Header.Get("Origin")); origin != "" {
			if _, allowed := origins[strings.ToLower(origin)]; !allowed {
				writeSecurityError(response, http.StatusForbidden, "FORBIDDEN")
				return
			}
		}
		authorization := request.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "Bearer ") {
			writeSecurityError(response, http.StatusUnauthorized, "UNAUTHENTICATED")
			return
		}
		token := strings.TrimPrefix(authorization, "Bearer ")
		digest := sha256.Sum256([]byte(token))
		if !validTokenShape(token) || subtle.ConstantTimeCompare(expectedDigest, digest[:]) != 1 {
			writeSecurityError(response, http.StatusUnauthorized, "UNAUTHENTICATED")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func validTokenShape(token string) bool {
	if !strings.HasPrefix(token, tokenPrefix) {
		return false
	}
	encoded := strings.TrimPrefix(token, tokenPrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(encoded, "="))
	return err == nil && len(decoded) == 32
}

func decodeDigest(value string) ([]byte, error) {
	if len(value) != sha256.Size*2 {
		return nil, fmt.Errorf("MCP token digest must be SHA-256: %w", searchruntime.ErrInvalidArgument)
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode MCP token digest: %w", searchruntime.ErrInvalidArgument)
	}
	return decoded, nil
}

func normalizedSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result[strings.ToLower(value)] = struct{}{}
		}
	}
	return result
}

func safeToolError(err error) error {
	switch {
	case errors.Is(err, searchruntime.ErrInvalidArgument):
		return errors.New("INVALID_ARGUMENT")
	case errors.Is(err, searchruntime.ErrProfileMismatch):
		return errors.New("PROFILE_MISMATCH")
	case searchruntime.IsRetryable(err):
		return errors.New("TEMPORARILY_UNAVAILABLE")
	default:
		return errors.New("INTERNAL")
	}
}

func writeSecurityError(response http.ResponseWriter, status int, code string) {
	if status == http.StatusUnauthorized {
		response.Header().Set("WWW-Authenticate", "Bearer")
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"error": map[string]any{
			"code":    -32000,
			"message": code,
			"data": map[string]any{
				"code":      code,
				"retryable": false,
			},
		},
	})
}
