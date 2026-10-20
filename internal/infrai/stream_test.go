package infrai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestPublishRequestBoundary(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", req.Method)
		}
		if req.URL.Path != "/v1/realtime/publish" {
			t.Fatalf("path = %s", req.URL.Path)
		}
		if req.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("missing bearer authorization")
		}
		if req.Header.Get("Idempotency-Key") != "batch-42" {
			t.Fatal("missing idempotency key")
		}
		body, _ := io.ReadAll(req.Body)
		got := string(body)
		for _, field := range []string{`"channel":"property-operations"`, `"event":"operations.snapshot"`, `"data":{"open":2}`} {
			if !strings.Contains(got, field) {
				t.Fatalf("body %s does not contain %s", got, field)
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true,"data":{},"error":null,"metadata":{}}`))}, nil
	})
	client := New("test-key")
	client.HTTP = &http.Client{Transport: transport}
	if err := client.Publish(context.Background(), "property-operations", "operations.snapshot", "batch-42", map[string]int{"open": 2}); err != nil {
		t.Fatal(err)
	}
}

func TestEnvelopeErrorPrecedesHTTPStatus(t *testing.T) {
	client := New("test-key")
	client.HTTP = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("name") != "property.operations" {
			t.Fatalf("name = %q, want property.operations", req.URL.Query().Get("name"))
		}
		if req.URL.Query().Get("agg") != "sum" {
			t.Fatalf("agg = %q, want sum", req.URL.Query().Get("agg"))
		}
		body := `{"ok":false,"data":null,"error":{"code":"REJECTED","message":"invalid input"},"metadata":{}}`
		return &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	_, err := client.QueryMetrics(context.Background())
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != "REJECTED" || apiErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("error = %#v", err)
	}
}
