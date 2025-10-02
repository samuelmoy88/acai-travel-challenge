package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go/v2"
)

// HolidaysTool provides information about local bank and public holidays
type HolidaysTool struct {
	calendarURL string
}

func NewHolidaysTool() *HolidaysTool {
	url := "https://www.officeholidays.com/ics/spain/catalonia"
	if v := os.Getenv("HOLIDAY_CALENDAR_LINK"); v != "" {
		url = v
	}
	return &HolidaysTool{calendarURL: url}
}

func (t *HolidaysTool) Name() string {
	return "get_holidays"
}

func (t *HolidaysTool) Description() string {
	return "Gets local bank and public holidays. Each line is a single holiday in the format 'YYYY-MM-DD: Holiday Name'."
}

func (t *HolidaysTool) Parameters() openai.FunctionParameters {
	return openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"before_date": map[string]string{
				"type":        "string",
				"description": "Optional date in RFC3339 format to get holidays before this date. If not provided, all holidays will be returned.",
			},
			"after_date": map[string]string{
				"type":        "string",
				"description": "Optional date in RFC3339 format to get holidays after this date. If not provided, all holidays will be returned.",
			},
			"max_count": map[string]string{
				"type":        "integer",
				"description": "Optional maximum number of holidays to return. If not provided, all holidays will be returned.",
			},
		},
	}
}

func (t *HolidaysTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args struct {
		BeforeDate string `json:"before_date,omitempty"`
		AfterDate  string `json:"after_date,omitempty"`
		MaxCount   int    `json:"max_count,omitempty"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Parse dates if provided
	var beforeDate, afterDate time.Time
	var err error

	if args.BeforeDate != "" {
		beforeDate, err = time.Parse(time.RFC3339, args.BeforeDate)
		if err != nil {
			return "", fmt.Errorf("invalid before_date: %w", err)
		}
	}

	if args.AfterDate != "" {
		afterDate, err = time.Parse(time.RFC3339, args.AfterDate)
		if err != nil {
			return "", fmt.Errorf("invalid after_date: %w", err)
		}
	}

	// Load calendar events
	events, err := LoadCalendar(ctx, t.calendarURL)
	if err != nil {
		return "Failed to load holiday events", err
	}

	// Filter and format holidays
	var holidays []string
	for _, event := range events {
		date, err := event.GetAllDayStartAt()
		if err != nil {
			continue
		}

		// Check max count
		if args.MaxCount > 0 && len(holidays) >= args.MaxCount {
			break
		}

		// Check date filters
		if !beforeDate.IsZero() && date.After(beforeDate) {
			continue
		}
		if !afterDate.IsZero() && date.Before(afterDate) {
			continue
		}

		holidays = append(holidays,
			date.Format(time.DateOnly)+": "+event.GetProperty("SUMMARY").Value)
	}

	return strings.Join(holidays, "\n"), nil
}
