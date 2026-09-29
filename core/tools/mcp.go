package tools

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	einoMCP "github.com/cloudwego/eino-ext/components/tool/mcp"
	einoTool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type McpConfig struct {
	URL            string
	Type           string
	Token          string
	CredentialType string
	ClientName     string
	ClientVersion  string
}

// NewMCPClient 根据配置创建并初始化 MCP 客户端。
// 支持 SSE 与 streamable_http 两种传输方式，并在失败时关闭已创建的连接。
func NewMCPClient(ctx context.Context, config *McpConfig) (*client.Client, error) {
	// 基础配置校验
	if config == nil {
		return nil, fmt.Errorf("MCP config is nil")
	}
	if strings.TrimSpace(config.URL) == "" {
		return nil, fmt.Errorf("MCP URL is empty")
	}
	// 仅允许 http/https，且禁止 URL 中携带用户信息，降低误配与凭证泄露风险
	parsedURL, parseErr := url.Parse(config.URL)
	if parseErr != nil || parsedURL.Host == "" || parsedURL.User != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, fmt.Errorf("invalid MCP URL")
	}

	// 按凭证类型组装请求头；当前仅支持 Bearer
	headers := make(map[string]string)
	if config.Token != "" {
		switch strings.ToLower(config.CredentialType) {
		case "", "bearer":
			headers["Authorization"] = fmt.Sprintf("Bearer %s", config.Token)
		default:
			return nil, fmt.Errorf("unsupported MCP credential type: %s", config.CredentialType)
		}
	}

	// 客户端标识：未配置时使用平台默认值
	clientName := config.ClientName
	if clientName == "" {
		clientName = "agent-platform"
	}
	clientVersion := config.ClientVersion
	if clientVersion == "" {
		clientVersion = "1.0.0"
	}

	var (
		cli *client.Client
		err error
	)

	// 按 type 选择传输实现；type 为空时做兼容推断
	switch {
	case strings.EqualFold(config.Type, "sse"):
		cli, err = client.NewSSEMCPClient(
			config.URL,
			transport.WithHeaders(headers),
		)
	case strings.EqualFold(config.Type, "streamable_http"):
		cli, err = client.NewStreamableHttpClient(
			config.URL,
			transport.WithHTTPHeaders(headers),
		)
	case config.Type == "" && strings.HasSuffix(strings.TrimRight(config.URL, "/"), "/sse"):
		// 兼容旧配置：未填写 type 且地址以 /sse 结尾时使用 SSE。
		cli, err = client.NewSSEMCPClient(
			config.URL,
			transport.WithHeaders(headers),
		)
	case config.Type == "":
		// 默认走 streamable_http
		cli, err = client.NewStreamableHttpClient(
			config.URL,
			transport.WithHTTPHeaders(headers),
		)
	default:
		return nil, fmt.Errorf("unsupported MCP type: %s", config.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("create MCP client: %w", err)
	}

	// 启动底层传输；失败时关闭客户端，避免资源泄漏
	if err := cli.Start(ctx); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("start MCP client: %w", err)
	}

	// 完成 MCP 协议握手，声明客户端信息与协议版本
	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	request.Params.ClientInfo = mcp.Implementation{
		Name:    clientName,
		Version: clientVersion,
	}
	if _, err := cli.Initialize(ctx, request); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("initialize MCP client: %w", err)
	}
	return cli, nil
}

func ListMCPTools(ctx context.Context, config *McpConfig) ([]mcp.Tool, *client.Client, error) {
	cli, err := NewMCPClient(ctx, config)
	if err != nil {
		return nil, nil, err
	}
	result, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		_ = cli.Close()
		return nil, nil, fmt.Errorf("list MCP tools: %w", err)
	}
	return result.Tools, cli, nil
}

func GetMCPTools(ctx context.Context, config *McpConfig) ([]einoTool.BaseTool, *client.Client, error) {
	cli, err := NewMCPClient(ctx, config)
	if err != nil {
		return nil, nil, err
	}
	// 使用 Eino 官方 MCP 适配器，保留远端工具的完整参数 schema 和调用语义。
	adapted, err := einoMCP.GetTools(ctx, &einoMCP.Config{Cli: cli})
	if err != nil {
		_ = cli.Close()
		return nil, nil, fmt.Errorf("adapt MCP tools: %w", err)
	}
	return adapted, cli, nil
}

// NamedTool 为模型侧工具提供唯一名称，底层仍调用 MCP Server 的原始工具名。
type NamedTool struct {
	Name     string
	Delegate einoTool.InvokableTool
}

func (t *NamedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	info, err := t.Delegate.Info(ctx)
	if err != nil {
		return nil, err
	}
	copyInfo := *info
	copyInfo.Name = t.Name
	return &copyInfo, nil
}

func (t *NamedTool) InvokableRun(ctx context.Context, arguments string, opts ...einoTool.Option) (string, error) {
	return t.Delegate.InvokableRun(ctx, arguments, opts...)
}
