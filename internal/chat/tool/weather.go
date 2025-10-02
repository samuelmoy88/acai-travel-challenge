package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go/v2"
)

type WeatherClient interface {
	GetCurrentWeather(ctx context.Context, location string) (string, error)
}

type WeatherTool struct {
	client WeatherClient
}

func NewWeatherTool(client WeatherClient) *WeatherTool {
	return &WeatherTool{client: client}
}

func (w *WeatherTool) Name() string {
	return "get_weather"
}

func (w *WeatherTool) Description() string {
	return "Get weather at the given location"
}

func (w *WeatherTool) Parameters() openai.FunctionParameters {
	return openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]string{
				"type":        "string",
				"description": "City name or location",
			},
		},
		"required": []string{"location"},
	}
}

func (t *WeatherTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args struct {
		Location string `json:"location"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	return t.client.GetCurrentWeather(ctx, args.Location)
}
