// Webhook signature verification, "Standard Webhooks" format (see docs/api.md "Webhooks" in the
// duva repository). Duva SENDS webhooks; this file is for VERIFYING them on your side.
package duva

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	webhookSecretPrefix     = "whsec_"
	defaultToleranceSeconds = 300
)

// WebhookEvent is the JSON body Duva sends to your webhook URL, already parsed.
type WebhookEvent struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Domain string         `json:"domain"`
	Data   map[string]any `json:"data"`
}

// VerifyOptions configures VerifyWebhookSignature and ConstructEvent. The zero value uses the
// defaults (5 minute tolerance, the real current time).
type VerifyOptions struct {
	// ToleranceSeconds: 0 means the default (300s).
	ToleranceSeconds int
	// Now: the current instant, for YOUR OWN tests only (see SignWebhookRequest and the fixtures
	// of duva-mail/duva-conformance, which document the exact instant each vector was signed at).
	// The zero value means time.Now().
	Now time.Time
}

// VerifyWebhookSignature verifies a webhook request. secrets accepts one or several (for key
// rotation: while both the old and the new secret are active, a webhook signed with either must
// verify).
//
// headers is looked up case-insensitively (http.Header always is). rawBody must be the EXACT
// bytes Duva sent: re-encoding a parsed-then-re-serialized JSON body changes its bytes and
// invalidates every signature (see your framework's raw-body option, documented per-framework in
// docs/api.md).
func VerifyWebhookSignature(secrets []string, headers http.Header, rawBody []byte, opts *VerifyOptions) bool {
	id := headers.Get("webhook-id")
	timestamp := headers.Get("webhook-timestamp")
	signatureHeader := headers.Get("webhook-signature")
	if id == "" || timestamp == "" || signatureHeader == "" {
		return false
	}

	at, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	tolerance := int64(defaultToleranceSeconds)
	now := time.Now().Unix()
	if opts != nil {
		if opts.ToleranceSeconds > 0 {
			tolerance = int64(opts.ToleranceSeconds)
		}
		if !opts.Now.IsZero() {
			now = opts.Now.Unix()
		}
	}
	if abs64(now-at) > tolerance {
		return false
	}

	expected := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		if sig, ok := expectedSignature(secret, id, timestamp, rawBody); ok {
			expected = append(expected, sig)
		}
	}

	// webhook-signature may carry several space-separated v1,<signature> entries (Duva sends one;
	// a sender that itself rotates its OWN signing key mid-flight could send more): any match
	// against any of your active secrets is accepted.
	for _, part := range strings.Split(signatureHeader, " ") {
		version, signature, found := strings.Cut(part, ",")
		if !found || version != "v1" || signature == "" {
			continue
		}
		for _, candidate := range expected {
			// Byte-for-byte comparison of the two base64-encoded TEXTS: two equal signatures are
			// identical as text too, so this never needs to decode (or trust) attacker input.
			if hmac.Equal([]byte(signature), []byte(candidate)) {
				return true
			}
		}
	}
	return false
}

// ConstructEvent: VerifyWebhookSignature, then parses the body. Returns a *WebhookSignatureError
// on a bad signature rather than a boolean, for call sites that want to fail closed. Never
// includes the secret or the raw body in the error.
func ConstructEvent(secrets []string, headers http.Header, rawBody []byte, opts *VerifyOptions) (*WebhookEvent, error) {
	if !VerifyWebhookSignature(secrets, headers, rawBody, opts) {
		return nil, &WebhookSignatureError{reason: "signature or timestamp check failed"}
	}
	var event WebhookEvent
	if err := json.Unmarshal(rawBody, &event); err != nil {
		return nil, &WebhookSignatureError{reason: "body is not valid JSON"}
	}
	return &event, nil
}

// SignWebhookRequest builds a validly signed request FOR YOUR OWN TESTS: the headers a real Duva
// webhook delivery would carry for body, signed with secret as of timestamp (the zero value means
// now). Never used by this package to send anything: Duva is the only real sender.
func SignWebhookRequest(secret, eventID string, body []byte, timestamp time.Time) http.Header {
	at := timestamp
	if at.IsZero() {
		at = time.Now()
	}
	ts := strconv.FormatInt(at.Unix(), 10)
	sig, _ := expectedSignature(secret, eventID, ts, body)
	headers := http.Header{}
	headers.Set("webhook-id", eventID)
	headers.Set("webhook-timestamp", ts)
	headers.Set("webhook-signature", "v1,"+sig)
	return headers
}

func expectedSignature(secret, id, timestamp string, rawBody []byte) (string, bool) {
	if !strings.HasPrefix(secret, webhookSecretPrefix) {
		return "", false
	}
	key, err := base64.StdEncoding.DecodeString(secret[len(webhookSecretPrefix):])
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + timestamp + "."))
	mac.Write(rawBody)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), true
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
