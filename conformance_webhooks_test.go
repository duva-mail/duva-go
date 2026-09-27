// Verified against duva-mail/duva-conformance (fetched by scripts/fetch-conformance.sh, never
// committed: see .gitignore). Run scripts/fetch-conformance.sh first if conformance/webhooks.json
// is missing.
package duva_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/duva-mail/duva-go"
)

type webhookFixture struct {
	ReferenceNow     string        `json:"reference_now"`
	ToleranceSeconds int           `json:"tolerance_seconds"`
	Secret           string        `json:"secret"`
	Cases            []webhookCase `json:"cases"`
}

type webhookCase struct {
	Name             string            `json:"name"`
	Category         string            `json:"category"`
	SignedWith       string            `json:"signed_with"`
	Headers          map[string]string `json:"headers"`
	Body             string            `json:"body"`
	Expect           bool              `json:"expect"`
	OtherValidSecret string            `json:"other_valid_secret"`
}

func loadWebhookFixture(t *testing.T) *webhookFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("conformance", "webhooks.json"))
	if err != nil {
		t.Skip("conformance/webhooks.json missing: run scripts/fetch-conformance.sh")
	}
	var fx webhookFixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("parsing conformance/webhooks.json: %v", err)
	}
	return &fx
}

func toHeader(m map[string]string) http.Header {
	h := http.Header{}
	for k, v := range m {
		h.Set(k, v)
	}
	return h
}

func referenceTime(t *testing.T, value string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parsing reference_now: %v", err)
	}
	return at
}

func findRotationCase(t *testing.T, fx *webhookFixture) webhookCase {
	t.Helper()
	for _, c := range fx.Cases {
		if c.Category == "rotation" {
			return c
		}
	}
	t.Fatal("no rotation case in fixture")
	return webhookCase{}
}

// TestWebhookConformance: the verifier must accept or reject EVERY vector exactly as expected,
// always with its OWN secret (the fixture's canonical one) -- never signed_with (informational
// only: which secret actually produced the signature, exactly what a wrong_secret case
// distinguishes).
func TestWebhookConformance(t *testing.T) {
	fx := loadWebhookFixture(t)
	now := referenceTime(t, fx.ReferenceNow)
	opts := &duva.VerifyOptions{ToleranceSeconds: fx.ToleranceSeconds, Now: now}

	nonRotation := 0
	for _, c := range fx.Cases {
		if c.Category == "rotation" {
			continue
		}
		nonRotation++
		c := c
		t.Run(c.Name, func(t *testing.T) {
			got := duva.VerifyWebhookSignature([]string{fx.Secret}, toHeader(c.Headers), []byte(c.Body), opts)
			if got != c.Expect {
				t.Fatalf("expected %v, got %v", c.Expect, got)
			}
		})
	}
	if nonRotation != len(fx.Cases)-1 {
		t.Fatalf("expected exactly one rotation case, found %d non-rotation of %d total", nonRotation, len(fx.Cases))
	}
}

// TestWebhookConformanceRotation: the active set during rotation is the current secret
// (OtherValidSecret, == the top-level secret) AND the older one that actually signed this
// webhook (SignedWith) -- a verifier trying only one at a time is exactly what the next test
// disproves.
func TestWebhookConformanceRotation(t *testing.T) {
	fx := loadWebhookFixture(t)
	opts := &duva.VerifyOptions{Now: referenceTime(t, fx.ReferenceNow)}
	rotation := findRotationCase(t, fx)

	got := duva.VerifyWebhookSignature(
		[]string{rotation.OtherValidSecret, rotation.SignedWith}, toHeader(rotation.Headers), []byte(rotation.Body), opts,
	)
	if got != rotation.Expect {
		t.Fatalf("expected %v, got %v", rotation.Expect, got)
	}
}

func TestWebhookConformanceRotationCurrentSecretAloneIsNotEnough(t *testing.T) {
	fx := loadWebhookFixture(t)
	opts := &duva.VerifyOptions{Now: referenceTime(t, fx.ReferenceNow)}
	rotation := findRotationCase(t, fx)

	if duva.VerifyWebhookSignature([]string{rotation.OtherValidSecret}, toHeader(rotation.Headers), []byte(rotation.Body), opts) {
		t.Fatal("expected the current secret alone to be insufficient")
	}
	// The point: a verifier must try the signer's secret too -- it doesn't know in advance which
	// one signed.
	if !duva.VerifyWebhookSignature([]string{rotation.SignedWith}, toHeader(rotation.Headers), []byte(rotation.Body), opts) {
		t.Fatal("expected the signer's own secret to verify alone")
	}
}
