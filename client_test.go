package duva_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/duva-mail/duva-go"
)

func TestNewRequiresAnAPIKeyAndDomain(t *testing.T) {
	if _, err := duva.New("", "example.com"); err == nil {
		t.Fatal("expected an error for a missing API key")
	}
	if _, err := duva.New("dv_test", ""); err == nil {
		t.Fatal("expected an error for a missing domain")
	}
}

func TestNewFallsBackToEnvironmentVariables(t *testing.T) {
	t.Setenv("DUVA_API_KEY", "dv_env")
	t.Setenv("DUVA_DOMAIN", "env.example.com")
	client, err := duva.New("", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if client == nil {
		t.Fatal("expected a client")
	}
}

func TestNewRejectsAnInsecureBaseURL(t *testing.T) {
	if _, err := duva.New("dv_test", "example.com", duva.WithBaseURL("http://api.example.com")); err == nil {
		t.Fatal("expected http:// (non-localhost) to be rejected")
	}
	if _, err := duva.New("dv_test", "example.com", duva.WithBaseURL("http://localhost:8080")); err != nil {
		t.Fatalf("expected http://localhost to be allowed: %v", err)
	}
}

func fakeClient(t *testing.T, handler func(*http.Request) (*http.Response, error)) (*duva.Client, *duva.TestTransport) {
	t.Helper()
	transport := duva.NewTestTransport(handler)
	client, err := duva.New("dv_test", "example.com", duva.WithHTTPClient(transport))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client, transport
}

func TestMessagesSendGeneratesAnIdempotencyKeyWhenNoneIsGiven(t *testing.T) {
	client, transport := fakeClient(t, func(req *http.Request) (*http.Response, error) {
		return duva.JSONResponse(202, map[string]any{"id": "msg_x", "status": "queued"}), nil
	})
	if _, err := client.Messages.Send(context.Background(), duva.SendMessageParams{
		From: "a@example.com", To: []string{"b@example.org"}, Subject: "s", Text: "t",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	key := transport.Requests[0].Header.Get("Idempotency-Key")
	if key == "" {
		t.Fatal("expected a generated idempotency key")
	}
	if _, err := client.Messages.Send(context.Background(), duva.SendMessageParams{
		From: "a@example.com", To: []string{"b@example.org"}, Subject: "s", Text: "t",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if transport.Requests[1].Header.Get("Idempotency-Key") == key {
		t.Fatal("expected a fresh key generated per call")
	}
}

func TestMessagesSendRejectsAnInvalidIdempotencyKeyLocally(t *testing.T) {
	client, transport := fakeClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatal("should not have sent a request")
		return nil, nil
	})
	_, err := client.Messages.Send(context.Background(), duva.SendMessageParams{
		From: "a@example.com", To: []string{"b@example.org"}, Subject: "s", Text: "t",
		IdempotencyKey: "has a space",
	})
	if err == nil {
		t.Fatal("expected a local validation error")
	}
	if len(transport.Requests) != 0 {
		t.Fatal("expected no request to have been sent")
	}
}

func TestMessagesSendReportsReplayedAndLocation(t *testing.T) {
	client, _ := fakeClient(t, func(req *http.Request) (*http.Response, error) {
		resp := duva.JSONResponse(202, map[string]any{"id": "msg_x", "status": "queued"})
		resp.Header.Set("Idempotent-Replayed", "true")
		resp.Header.Set("Location", "https://api.duva.ca/v1/example.com/messages/msg_x")
		return resp, nil
	})
	result, err := client.Messages.Send(context.Background(), duva.SendMessageParams{
		From: "a@example.com", To: []string{"b@example.org"}, Subject: "s", Text: "t",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !result.Replayed {
		t.Error("expected Replayed to be true")
	}
	if result.Location != "https://api.duva.ca/v1/example.com/messages/msg_x" {
		t.Errorf("unexpected Location: %s", result.Location)
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		code   string
		check  func(error) bool
	}{
		{401, "unauthorized", func(err error) bool { _, ok := err.(*duva.AuthenticationError); return ok }},
		{404, "not_found", func(err error) bool { _, ok := err.(*duva.NotFoundError); return ok }},
		{403, "domain_not_verified", func(err error) bool { _, ok := err.(*duva.PermissionError); return ok }},
		{409, "limit_reached", func(err error) bool { _, ok := err.(*duva.ConflictError); return ok }},
		{413, "payload_too_large", func(err error) bool { _, ok := err.(*duva.PayloadTooLargeError); return ok }},
		{422, "invalid_request", func(err error) bool { _, ok := err.(*duva.ValidationError); return ok }},
		{500, "internal_error", func(err error) bool { _, ok := err.(*duva.ServerError); return ok }},
		{502, "unknown_future_code", func(err error) bool { _, ok := err.(*duva.ServerError); return ok }},
	}
	for _, c := range cases {
		c := c
		t.Run(c.code, func(t *testing.T) {
			client, _ := fakeClient(t, func(req *http.Request) (*http.Response, error) {
				return duva.JSONResponse(c.status, map[string]any{
					"error": map[string]any{"code": c.code, "message": "x"},
				}), nil
			})
			_, err := client.Messages.Get(context.Background(), "msg_x")
			if err == nil {
				t.Fatal("expected an error")
			}
			if !c.check(err) {
				t.Fatalf("unexpected error type: %#v", err)
			}
			if got, ok := duva.ErrorCode(err); !ok || got != c.code {
				t.Fatalf("ErrorCode: got %q, %v", got, ok)
			}
		})
	}
}

func TestValidationErrorCarriesFields(t *testing.T) {
	client, _ := fakeClient(t, func(req *http.Request) (*http.Response, error) {
		return duva.JSONResponse(422, map[string]any{
			"error": map[string]any{
				"code": "invalid_request", "message": "x",
				"fields": []any{map[string]any{"field": "to[0]", "message": "invalid"}},
			},
		}), nil
	})
	_, err := client.Messages.Get(context.Background(), "msg_x")
	if err == nil {
		t.Fatal("expected an error")
	}
	validationErr, ok := err.(*duva.ValidationError)
	if !ok {
		t.Fatalf("expected *duva.ValidationError, got %#v", err)
	}
	if len(validationErr.Fields) != 1 || validationErr.Fields[0].Field != "to[0]" {
		t.Fatalf("unexpected fields: %#v", validationErr.Fields)
	}
}

func TestQuotaExceededIsNeverRetried(t *testing.T) {
	calls := 0
	client, _ := fakeClient(t, func(req *http.Request) (*http.Response, error) {
		calls++
		return duva.JSONResponse(429, map[string]any{
			"error": map[string]any{"code": "quota_exceeded", "message": "x"},
		}), nil
	})
	_, err := client.Messages.Send(context.Background(), duva.SendMessageParams{
		From: "a@example.com", To: []string{"b@example.org"}, Subject: "s", Text: "t",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, ok := err.(*duva.QuotaExceededError); !ok {
		t.Fatalf("expected *duva.QuotaExceededError, got %#v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one attempt, got %d", calls)
	}
}
