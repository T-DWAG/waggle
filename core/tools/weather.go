package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const weatherToolName = "get_weather"

// WeatherTool 使用高德天气 API 查询城市天气。
type WeatherTool struct {
	apiKey string
	client *http.Client
}

type WeatherConfig struct {
	APIKey string `json:"apiKey"`
}

func NewWeatherTool(config *WeatherConfig) InvokeParamTool {
	if config == nil {
		config = &WeatherConfig{}
	}
	return &WeatherTool{
		apiKey: strings.TrimSpace(config.APIKey),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WeatherTool) Params() map[string]*schema.ParameterInfo {
	return map[string]*schema.ParameterInfo{
		"city": {
			Desc:     "需要查询天气的城市名称或区域编码",
			Type:     schema.String,
			Required: true,
		},
		"extensions": {
			Desc: "气象类型: base(实况天气) / all(预报天气)",
			Type: schema.String,
			Enum: []string{"base", "all"},
		},
	}
}

func (w *WeatherTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        weatherToolName,
		Desc:        "查询指定城市的天气信息，使用高德天气API。用户询问天气、气温、预报时调用。",
		ParamsOneOf: schema.NewParamsOneOfByParams(w.Params()),
	}, nil
}

func (w *WeatherTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if w.apiKey == "" {
		return "", fmt.Errorf("高德天气 API Key 未配置")
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}
	city, _ := params["city"].(string)
	city = strings.TrimSpace(city)
	if city == "" {
		return "", fmt.Errorf("city is required")
	}

	query := url.Values{}
	query.Set("city", city)
	query.Set("key", w.apiKey)
	query.Set("output", "JSON")
	if extensions, ok := params["extensions"].(string); ok && (extensions == "base" || extensions == "all") {
		query.Set("extensions", extensions)
	} else {
		query.Set("extensions", "base")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://restapi.amap.com/v3/weather/weatherInfo?"+query.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	response, err := w.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", response.StatusCode, string(body))
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return string(body), nil
	}
	if status, _ := payload["status"].(string); status != "" && status != "1" {
		info, _ := payload["info"].(string)
		if info == "" {
			info = "高德天气查询失败"
		}
		return "", fmt.Errorf("%s", info)
	}
	return string(body), nil
}
