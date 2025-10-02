package tool

import (
	"context"
	"time"

	"github.com/openai/openai-go/v2"
)

// DateTool provides current date and time information
type DateTool struct{}

func NewDateTool() *DateTool {
	return &DateTool{}
}

func (t *DateTool) Name() string {
	return "get_today_date"
}

func (t *DateTool) Description() string {
	return "Get today's date and time in RFC3339 format"
}

func (t *DateTool) Parameters() openai.FunctionParameters {
	return openai.FunctionParameters{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *DateTool) Execute(ctx context.Context, arguments string) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}
