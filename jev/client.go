package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RetryPolicy controls retries after the initial HTTP attempt.
// MaxRetries zero disables retries. A nil Config.Retry uses two retries.
type RetryPolicy struct {
	MaxRetries int
}

// Config sets client defaults. The caller supplies APIKey; the client does not
// read environment variables. HTTPClient is optional and is not modified.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	Timeout    time.Duration
	Retry      *RetryPolicy
	HTTPClient *http.Client
}

// Client is safe for concurrent calls. Callers must not change input maps or
// an injected HTTP client's settings during a call.
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	timeout    time.Duration
	maxRetries int
	httpClient *http.Client
}

// New creates a client without making a network request.
// Defaults are https://api.typesafe.ai, jev-latest, and a ten-second total timeout.
func New(config Config) (*Client, error) {
	if strings.TrimSpace(config.APIKey) == "" || strings.ContainsAny(config.APIKey, "\r\n") {
		return nil, fmt.Errorf("jev: API key is missing or invalid")
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.typesafe.ai"
	}

	if config.Model == "" {
		config.Model = "jev-latest"
	}

	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.Timeout < 0 {
		return nil, fmt.Errorf("jev: timeout must be positive")
	}

	base, err := url.Parse(config.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, fmt.Errorf("jev: invalid base URL")
	}

	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{}
	}

	retries := 2
	if config.Retry != nil {
		retries = config.Retry.MaxRetries
	}
	if retries < 0 {
		return nil, fmt.Errorf("jev: retry count must not be negative")
	}

	return &Client{
		apiKey: config.APIKey, baseURL: strings.TrimRight(config.BaseURL, "/"),
		model: config.Model, timeout: config.Timeout, maxRetries: retries,
		httpClient: config.HTTPClient,
	}, nil
}

// SystemOne evaluates all questions against the same state.
func (c *Client) SystemOne(ctx context.Context, request Request) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if request.Model == "" {
		request.Model = c.model
	}

	body, err := json.Marshal(request)
	if err != nil {
		return Response{}, fmt.Errorf("jev: cannot encode request")
	}
	questions, err := validateRequest(body)
	if err != nil {
		return Response{}, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/systemone", body)
	if err != nil {
		return Response{}, err
	}

	return decodeResponse(data, questions)
}

// ListModels lists names available to the account. Versioned model IDs may be
// accepted by SystemOne even when they are absent from this list.
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	data, err := c.do(ctx, http.MethodGet, "/v1/models", nil)
	if err != nil {
		return nil, err
	}

	return decodeModels(data)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("jev: cannot create HTTP request")
		}

		request.Header.Set("Authorization", "Bearer "+c.apiKey)
		request.Header.Set("Accept", "application/json")
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}

		response, err := c.httpClient.Do(request)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			return nil, fmt.Errorf("jev: HTTP request failed: %w", err)
		}

		data, readErr := readResponse(response)
		if readErr != nil {
			_ = response.Body.Close()
			return nil, readErr
		}

		if response.StatusCode == http.StatusOK {
			_ = response.Body.Close()
			return data, nil
		}

		apiErr := apiError(response, data)
		delay, retry := c.retryDelay(response.Header, response.StatusCode, attempt)
		_ = response.Body.Close()
		if !retry {
			return nil, apiErr
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *Client) retryDelay(header http.Header, status, attempt int) (time.Duration, bool) {
	if attempt >= c.maxRetries || !retryable(status) {
		return 0, false
	}

	if delay := headerDelay(header); delay >= 0 {
		return delay, true
	}

	delay := 250 * time.Millisecond * time.Duration(1<<min(attempt, 3))
	if delay > 2*time.Second {
		delay = 2 * time.Second
	}

	return delay, true
}

func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable,
		http.StatusGatewayTimeout, 529:
		return true
	default:
		return false
	}
}

func headerDelay(header http.Header) time.Duration {
	if value := header.Get("Retry-After-Ms"); value != "" {
		milliseconds, err := strconv.ParseInt(value, 10, 64)
		if err == nil && milliseconds >= 0 && milliseconds <= int64((1<<63-1)/int64(time.Millisecond)) {
			return time.Duration(milliseconds) * time.Millisecond
		}
	}

	if value := header.Get("Retry-After"); value != "" {
		if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((1<<63-1)/int64(time.Second)) {
			return time.Duration(seconds) * time.Second
		}
		if date, err := http.ParseTime(value); err == nil {
			delay := time.Until(date)
			if delay > 0 {
				return delay
			}
			return 0
		}
	}

	return -1
}
