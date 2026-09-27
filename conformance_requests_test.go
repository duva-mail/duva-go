// Verified against duva-mail/duva-conformance (fetched by scripts/fetch-conformance.sh, never
// committed: see .gitignore). Run scripts/fetch-conformance.sh first if conformance/requests.json
// is missing.
package duva_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/duva-mail/duva-go"
)

type requestFixture struct {
	Cases []requestCase `json:"cases"`
}

type requestCase struct {
	OperationID     string         `json:"operation_id"`
	Input           map[string]any `json:"input"`
	ExpectedRequest struct {
		Method  string            `json:"method"`
		Path    string            `json:"path"`
		Headers map[string]string `json:"headers"`
		Body    any               `json:"body"`
	} `json:"expected_request"`
}

func loadRequestFixture(t *testing.T) *requestFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("conformance", "requests.json"))
	if err != nil {
		t.Skip("conformance/requests.json missing: run scripts/fetch-conformance.sh")
	}
	var fx requestFixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("parsing conformance/requests.json: %v", err)
	}
	return &fx
}

func str(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func toStrSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, len(raw))
	for i, r := range raw {
		out[i], _ = r.(string)
	}
	return out
}

func toStrMap(v any) map[string]string {
	raw, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, r := range raw {
		out[k], _ = r.(string)
	}
	return out
}

// A generic stub covering every model's required fields at once (Message, Suppression,
// Webhook...): only the request that was SENT matters to this test (see below), so a superset
// body avoids a decode failure from whichever model happens to parse it, rather than one body
// per operation.
var requestStub = map[string]any{
	"id": "x", "status": "queued", "data": []any{},
	"email": "x@example.com", "reason": "manual", "created_at": "2026-01-01T00:00:00Z",
	"url": "https://example.org/hook", "events": []any{},
	"from": "x@example.com", "subject": "s", "tags": []any{}, "metadata": map[string]any{},
	"tracking": map[string]any{"opens": false, "clicks": false}, "recipients": []any{},
}

func callForRequest(t *testing.T, operationID string, client *duva.Client, input map[string]any) {
	t.Helper()
	ctx := context.Background()
	switch operationID {
	case "sendMessage":
		params := duva.SendMessageParams{
			From: str(input, "from_"), To: toStrSlice(input["to"]), Subject: str(input, "subject"),
			Text: str(input, "text"), Tags: toStrSlice(input["tags"]), Metadata: toStrMap(input["metadata"]),
			IdempotencyKey: str(input, "idempotency_key"),
		}
		//nolint:errcheck // only the request that was SENT matters to this test
		_, _ = client.Messages.Send(ctx, params)
	case "addSuppression":
		_, _, _ = client.Suppressions.Add(ctx, str(input, "email"))
	case "createWebhook":
		_, _ = client.Webhooks.Create(ctx, duva.CreateWebhookParams{URL: str(input, "url"), Events: toStrSlice(input["events"])})
	case "getMessage":
		_, _ = client.Messages.Get(ctx, str(input, "id"))
	case "removeSuppression":
		_ = client.Suppressions.Remove(ctx, str(input, "email"))
	default:
		t.Fatalf("no driver for %s", operationID)
	}
}

func TestRequestConformance(t *testing.T) {
	fx := loadRequestFixture(t)
	covered := map[string]bool{}
	for _, c := range fx.Cases {
		c := c
		t.Run(c.OperationID, func(t *testing.T) {
			transport := duva.NewTestTransport(func(req *http.Request) (*http.Response, error) {
				return duva.JSONResponse(200, requestStub), nil
			})
			client, err := duva.New(str(c.Input, "api_key"), str(c.Input, "domain"), duva.WithHTTPClient(transport))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			callForRequest(t, c.OperationID, client, c.Input)
			covered[c.OperationID] = true

			if len(transport.Requests) != 1 {
				t.Fatalf("expected exactly one request, got %d", len(transport.Requests))
			}
			got := transport.Requests[0]
			if got.Method != c.ExpectedRequest.Method {
				t.Errorf("method: got %s want %s", got.Method, c.ExpectedRequest.Method)
			}
			if got.Path != c.ExpectedRequest.Path {
				t.Errorf("path: got %s want %s", got.Path, c.ExpectedRequest.Path)
			}
			for name, want := range c.ExpectedRequest.Headers {
				if have := got.Header.Get(name); have != want {
					t.Errorf("header %s: got %q want %q", name, have, want)
				}
			}
			if c.ExpectedRequest.Body == nil {
				if len(got.Body) != 0 {
					t.Errorf("expected no body, got %s", got.Body)
				}
			} else {
				var gotBody any
				if err := json.Unmarshal(got.Body, &gotBody); err != nil {
					t.Fatalf("body not JSON: %v", err)
				}
				if !reflect.DeepEqual(gotBody, c.ExpectedRequest.Body) {
					t.Errorf("body mismatch:\n got  %#v\n want %#v", gotBody, c.ExpectedRequest.Body)
				}
			}
		})
	}

	for _, opID := range []string{"sendMessage", "addSuppression", "createWebhook", "getMessage", "removeSuppression"} {
		if !covered[opID] {
			t.Errorf("fixture no longer covers operation %s", opID)
		}
	}
}
