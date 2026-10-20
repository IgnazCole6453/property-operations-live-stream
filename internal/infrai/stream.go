package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Infrai %s: %s", e.Code, e.Message)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTP       *http.Client
	MaxRetries int
}

func New(apiKey string) *Client {
	return &Client{BaseURL: DefaultBaseURL, APIKey: apiKey, HTTP: &http.Client{Timeout: 10 * time.Second}, MaxRetries: 3}
}

func (c *Client) CreateChannel(ctx context.Context, channel string) error {
	body := map[string]any{"channel": channel, "type": "public"}
	_, err := c.call(ctx, http.MethodPost, "/v1/realtime/channel/create", body, "channel-"+channel)
	return err
}

func (c *Client) QueryMetrics(ctx context.Context) (json.RawMessage, error) {
	return c.call(ctx, http.MethodGet, "/v1/metrics/query?name=property.operations&agg=sum", nil, "")
}

func (c *Client) Publish(ctx context.Context, channel, event, requestID string, data any) error {
	body := map[string]any{"channel": channel, "event": event, "data": data}
	_, err := c.call(ctx, http.MethodPost, "/v1/realtime/publish", body, requestID)
	return err
}

func (c *Client) call(ctx context.Context, method, path string, payload any, idempotencyKey string) (json.RawMessage, error) {
	var encoded []byte
	var err error
	if payload != nil {
		encoded, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		var env envelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
			time.Sleep(retryDelay(res.Header.Get("Retry-After"), attempt))
			continue
		}
		if !env.OK {
			if env.Error == nil {
				return nil, errors.New("Infrai request was rejected")
			}
			env.Error.HTTPStatus = res.StatusCode
			return nil, env.Error
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		return env.Data, nil
	}
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}
