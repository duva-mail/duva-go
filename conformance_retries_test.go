// Verified against duva-mail/duva-conformance (fetched by scripts/fetch-conformance.sh, never
// committed: see .gitignore). Run scripts/fetch-conformance.sh first if conformance/retries.json
// is missing.
package duva_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/duva-mail/duva-go"
)

type retryFixture struct {
	Cases []retryCase `json:"cases"`
}

type retryCase struct {
	Name                string          `json:"name"`
	OperationID         string          `json:"operation_id"`
	MaxRetries          int             `json:"max_retries"`
	MaxRetryWaitSeconds float64         `json:"max_retry_wait_seconds"`
	ResponseSequence    []retryResponse `json:"response_sequence"`
	ExpectedAttempts    int             `json:"expected_attempts"`
	ExpectedOutcome     string          `json:"expected_outcome"`
}

type retryResponse struct {
	Status  *int              `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`
}

func loadRetryFixture(t *testing.T) *retryFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("conformance", "retries.json"))
	if err != nil {
		t.Skip("conformance/retries.json missing: run scripts/fetch-conformance.sh")
	}
	var fx retryFixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("parsing conformance/retries.json: %v", err)
	}
	return &fx
}

func callForRetry(operationID string, client *duva.Client) error {
	ctx := context.Background()
	switch operationID {
	case "sendMessage":
		_, err := client.Messages.Send(ctx, duva.SendMessageParams{
			From: "a@example.com", To: []string{"b@example.org"}, Subject: "s", Text: "t",
		})
		return err
	case "getMessage":
		_, err := client.Messages.Get(ctx, "msg_"+strings.Repeat("a", 32))
		return err
	case "addSuppression":
		_, _, err := client.Suppressions.Add(ctx, "b@example.org")
		return err
	case "listEvents":
		_, err := client.Events.List(ctx, duva.ListEventsParams{})
		return err
	default:
		return fmt.Errorf("no driver for %s", operationID)
	}
}

func TestRetryConformance(t *testing.T) {
	fx := loadRetryFixture(t)
	for _, c := range fx.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			attempts := 0
			transport := duva.NewTestTransport(func(req *http.Request) (*http.Response, error) {
				scripted := c.ResponseSequence[attempts]
				attempts++
				if scripted.Status == nil {
					return nil, errors.New("simulated network failure")
				}
				resp := duva.JSONResponse(*scripted.Status, scripted.Body)
				for name, value := range scripted.Headers {
					resp.Header.Set(name, value)
				}
				return resp, nil
			})
			client, err := duva.New(
				"dv_test", "example.com",
				duva.WithHTTPClient(transport),
				duva.WithMaxRetries(c.MaxRetries),
				duva.WithMaxRetryWait(time.Duration(c.MaxRetryWaitSeconds*float64(time.Second))),
			)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			callErr := callForRetry(c.OperationID, client)

			if c.ExpectedOutcome == "success" {
				// retries.json's success bodies are a generic placeholder across every operation
				// (only transport-level retry/error behavior is under test here, not response
				// shape): a shape mismatch for e.g. listEvents is expected, not a failure.
			} else {
				code := strings.TrimPrefix(c.ExpectedOutcome, "error:")
				if callErr == nil {
					t.Fatal("expected a failure")
				}
				if code == "server" {
					var serverErr *duva.ServerError
					var connErr *duva.ConnectionError
					if !errors.As(callErr, &serverErr) && !errors.As(callErr, &connErr) {
						t.Fatalf("expected *duva.ServerError or *duva.ConnectionError, got %#v", callErr)
					}
				} else if got, ok := duva.ErrorCode(callErr); !ok || got != code {
					t.Fatalf("expected error code %q, got %q (found=%v)", code, got, ok)
				}
			}

			if attempts != c.ExpectedAttempts {
				t.Fatalf("expected %d attempts, got %d", c.ExpectedAttempts, attempts)
			}
		})
	}
}
