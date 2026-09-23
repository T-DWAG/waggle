package tools

import "github.com/mszlu521/thunder/config"

// RegisterBuiltins 注册当前章节的内置工具。Key 为空时工具仍可入库，但测试和会话调用会返回明确错误。
func RegisterBuiltins() {
	RegisterSystemTools(NewWeatherTool(&WeatherConfig{APIKey: config.GetString("weather.apiKey")}))
}
