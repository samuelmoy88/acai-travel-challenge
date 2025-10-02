package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type WeatherClient struct {
	apiKey     string
	httpClient *http.Client
}

type WeatherResponse struct {
	Location struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"location"`
	Current struct {
		TempC     float64 `json:"temp_c"`
		IsDay     int     `json:"is_day"`
		Condition struct {
			Text string `json:"text"`
		} `json:"condition"`
		WindKph    float64 `json:"wind_kph"`
		WindDir    string  `json:"wind_dir"`
		Humidity   int     `json:"humidity"`
		Cloud      int     `json:"cloud"`
		FeelsLikeC float64 `json:"feelslike_c"`
		PrecipMm   float64 `json:"precip_mm"`
		UV         float64 `json:"uv"`
	} `json:"current"`
}

func NewWeatherClient() *WeatherClient {
	return &WeatherClient{
		apiKey: os.Getenv("WEATHER_API_KEY"),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (w *WeatherClient) GetCurrentWeather(ctx context.Context, location string) (string, error) {
	if w.apiKey == "" {
		return "Weather API key not configured", fmt.Errorf("WEATHER_API_KEY not set")
	}

	var lastErr error
	maxRetries := 3
	baseDelay := 200 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 200ms, 400ms, 800ms
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		weather, err := w.fetchWeather(ctx, location)
		if err == nil {
			return weather, nil
		}
		lastErr = err
	}

	return "Unable to fetch weather data at the moment. The weather is probably fine though! 🌤️", lastErr
}

func (w *WeatherClient) fetchWeather(ctx context.Context, location string) (string, error) {
	url := fmt.Sprintf(
		"http://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no",
		w.apiKey,
		location,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var weatherResp WeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&weatherResp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	// More detailed formatting
	timeOfDay := "during the day"
	if weatherResp.Current.IsDay == 0 {
		timeOfDay = "at night"
	}

	result := fmt.Sprintf(
		"Current weather in %s, %s (%s):\n"+
			"Temperature: %.1f°C (feels like %.1f°C)\n"+
			"Conditions: %s\n"+
			"Wind: %.1f km/h from %s\n"+
			"Humidity: %d%%\n"+
			"Cloud coverage: %d%%",
		weatherResp.Location.Name,
		weatherResp.Location.Country,
		timeOfDay,
		weatherResp.Current.TempC,
		weatherResp.Current.FeelsLikeC,
		weatherResp.Current.Condition.Text,
		weatherResp.Current.WindKph,
		weatherResp.Current.WindDir,
		weatherResp.Current.Humidity,
		weatherResp.Current.Cloud,
	)

	// Add precipitation if it's raining
	if weatherResp.Current.PrecipMm > 0 {
		result += fmt.Sprintf("\nPrecipitation: %.1f mm", weatherResp.Current.PrecipMm)
	}

	// Add UV warning if high
	if weatherResp.Current.UV > 6 {
		result += fmt.Sprintf("\n High UV index: %.0f (use sun protection)", weatherResp.Current.UV)
	}

	return result, nil
}
