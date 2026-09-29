package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	einoTool "github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func newEchoMCPServer() *server.MCPServer {
	s := server.NewMCPServer("test-server", "1.0.0", server.WithToolCapabilities(true))
	s.AddTool(mcp.NewTool("echo",
		mcp.WithDescription("echo text"),
		mcp.WithString("text", mcp.Required()),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("echo:" + request.GetString("text", "")), nil
	})
	return s
}

// requireBearer 校验 Token 是否按 Bearer 方式透传给远端。
func requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TestMCPTransports(t *testing.T) {
	streamable := httptest.NewServer(requireBearer(server.NewStreamableHTTPServer(newEchoMCPServer())))
	defer streamable.Close()

	sseServer := server.NewSSEServer(newEchoMCPServer())
	sse := httptest.NewServer(requireBearer(sseServer))
	defer sse.Close()

	cases := []struct {
		name   string
		config *McpConfig
	}{
		{"streamable_http", &McpConfig{URL: streamable.URL + "/mcp", Type: "streamable_http", Token: "secret", CredentialType: "bearer"}},
		{"default_streamable_http", &McpConfig{URL: streamable.URL + "/mcp", Token: "secret"}},
		{"sse", &McpConfig{URL: sse.URL + "/sse", Type: "sse", Token: "secret", CredentialType: "bearer"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			listed, cli, err := ListMCPTools(ctx, tc.config)
			if err != nil {
				t.Fatalf("list tools: %v", err)
			}
			_ = cli.Close()
			if len(listed) != 1 || listed[0].Name != "echo" {
				t.Fatalf("unexpected tools: %+v", listed)
			}

			adapted, cli, err := GetMCPTools(ctx, tc.config)
			if err != nil {
				t.Fatalf("get tools: %v", err)
			}
			defer cli.Close()
			invokable := adapted[0].(einoTool.InvokableTool)
			named := &NamedTool{Name: "mcp_test_echo", Delegate: invokable}
			info, err := named.Info(ctx)
			if err != nil || info.Name != "mcp_test_echo" {
				t.Fatalf("named info: %v %+v", err, info)
			}
			result, err := named.InvokableRun(ctx, `{"text":"hi"}`)
			if err != nil || !strings.Contains(result, "echo:hi") {
				t.Fatalf("invoke: %v %s", err, result)
			}
		})
	}
}

func TestMCPConfigValidation(t *testing.T) {
	ctx := context.Background()
	bad := []*McpConfig{
		nil,
		{URL: ""},
		{URL: "ftp://example.com"},
		{URL: "http://user:pass@example.com/mcp"},
		{URL: "http://example.com/mcp", Type: "stdio"},
		{URL: "http://example.com/mcp", Token: "x", CredentialType: "basic"},
	}
	for _, config := range bad {
		if _, err := NewMCPClient(ctx, config); err == nil {
			t.Fatalf("expected error for %+v", config)
		}
	}
}

func TestMCPAuthFailureDoesNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(requireBearer(server.NewStreamableHTTPServer(newEchoMCPServer())))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := NewMCPClient(ctx, &McpConfig{URL: srv.URL + "/mcp", Token: "wrong-token"})
	if err == nil {
		t.Fatal("expected auth failure")
	}
	if strings.Contains(err.Error(), "wrong-token") {
		t.Fatalf("token leaked in error: %v", err)
	}
}
