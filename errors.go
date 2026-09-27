package duva

import (
	"fmt"
	"strconv"
	"time"
)

// ValidationField is one entry of a *ValidationError's Fields (e.g. {Field: "to[0]", Message:
// "..."}).
type ValidationField struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// APIError is the common shape of every error Duva actually answered (as opposed to a network
// failure: see *ConnectionError and *TimeoutError). Every typed error below embeds one: use
// errors.As with the specific type (e.g. *NotFoundError) to match it, or compare .Code directly
// for a code this package doesn't have a dedicated type for yet.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Fields     []ValidationField
	RawBody    string // bounded to 4 KB; never your API key
}

func (e *APIError) Error() string { return fmt.Sprintf("duva: %s (%s)", e.Message, e.Code) }

// apiError is promoted to every typed error below through embedding, letting code that doesn't
// know the exact type (the retry logic; ErrorCode) recognize "some error Duva actually answered"
// and recover its common fields.
func (e *APIError) apiError() *APIError { return e }

type hasAPIError interface{ apiError() *APIError }

// ErrorCode returns err's error.code when err is one of this package's typed API errors (see
// APIError), and whether it was found. Useful to branch on the code without an errors.As for
// every possible type.
func ErrorCode(err error) (string, bool) {
	if e, ok := err.(hasAPIError); ok {
		return e.apiError().Code, true
	}
	return "", false
}

func newAPIError(status int, code, message, rawBody string, fields []ValidationField) *APIError {
	if len(rawBody) > 4096 {
		rawBody = rawBody[:4096]
	}
	return &APIError{StatusCode: status, Code: code, Message: message, Fields: fields, RawBody: rawBody}
}

// AuthenticationError: 401 unauthorized.
type AuthenticationError struct{ *APIError }

// NotFoundError: 404 not_found.
type NotFoundError struct{ *APIError }

// PermissionError: 403 domain_not_verified or sending_not_allowed (.Code distinguishes the two).
type PermissionError struct{ *APIError }

// ConflictError: 409 idempotency_conflict or limit_reached.
type ConflictError struct{ *APIError }

// PayloadTooLargeError: 413 payload_too_large.
type PayloadTooLargeError struct{ *APIError }

// ValidationError: 422 invalid_request. .Fields lists the offending fields.
type ValidationError struct{ *APIError }

// ServerError: a 5xx, or a response whose body was not the documented error envelope.
type ServerError struct{ *APIError }

// QuotaExceededError: a 429 on YOUR ACCOUNT quota (daily or monthly). RetryAfter can be hours:
// never retried automatically, by design (see docs/bibliotheques-clientes.md section 3.5 in the
// duva repository).
type QuotaExceededError struct {
	*APIError
	RetryAfter time.Duration
}

// RateLimitError: a 429 from the per-key rate limit (unrelated to your sending quota). Retried
// automatically when RetryAfter fits within the client's WithMaxRetryWait.
type RateLimitError struct {
	*APIError
	RetryAfter time.Duration
}

// ConnectionError: no response was received at all (DNS, TLS, connection refused, reset...).
type ConnectionError struct{ Err error }

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("duva: the request could not be sent: %v", e.Err)
}
func (e *ConnectionError) Unwrap() error { return e.Err }

// TimeoutError: the request exceeded the configured timeout before any response arrived.
type TimeoutError struct{ Err error }

func (e *TimeoutError) Error() string { return fmt.Sprintf("duva: request timed out: %v", e.Err) }
func (e *TimeoutError) Unwrap() error { return e.Err }

// WebhookSignatureError: a webhook signature failed to verify. Never carries the secret or body.
type WebhookSignatureError struct{ reason string }

func (e *WebhookSignatureError) Error() string {
	return "duva: webhook signature verification failed: " + e.reason
}

var errorCodeClass = map[string]func(*APIError) error{
	"unauthorized":         func(e *APIError) error { return &AuthenticationError{e} },
	"not_found":            func(e *APIError) error { return &NotFoundError{e} },
	"domain_not_verified":  func(e *APIError) error { return &PermissionError{e} },
	"sending_not_allowed":  func(e *APIError) error { return &PermissionError{e} },
	"idempotency_conflict": func(e *APIError) error { return &ConflictError{e} },
	"limit_reached":        func(e *APIError) error { return &ConflictError{e} },
	"payload_too_large":    func(e *APIError) error { return &PayloadTooLargeError{e} },
	"invalid_request":      func(e *APIError) error { return &ValidationError{e} },
	"internal_error":       func(e *APIError) error { return &ServerError{e} },
	"method_not_allowed":   func(e *APIError) error { return &ServerError{e} },
	"http_error":           func(e *APIError) error { return &ServerError{e} },
}

// errorFromResponse builds the right typed error from a parsed response body, or a generic
// *ServerError when the body does not match the documented envelope (a proxy error page, for
// instance): never a plain error.
func errorFromResponse(status int, parsed map[string]any, rawBody, retryAfterHeader string) error {
	code, message, fields := asErrorBody(parsed)
	retryAfterSeconds := 0
	if retryAfterHeader != "" {
		if n, err := strconv.Atoi(retryAfterHeader); err == nil {
			retryAfterSeconds = n
		}
	}
	base := newAPIError(status, code, message, rawBody, fields)
	switch code {
	case "quota_exceeded":
		return &QuotaExceededError{base, time.Duration(retryAfterSeconds) * time.Second}
	case "rate_limited":
		return &RateLimitError{base, time.Duration(retryAfterSeconds) * time.Second}
	}
	if build, ok := errorCodeClass[code]; ok {
		return build(base)
	}
	return &ServerError{base}
}

func asErrorBody(parsed map[string]any) (code, message string, fields []ValidationField) {
	errObj, ok := asErrorObject(parsed)
	if !ok {
		return "http_error", "Duva answered with an unexpected body.", nil
	}
	code, _ = errObj["code"].(string)
	if code == "" {
		return "http_error", "Duva answered with an unexpected body.", nil
	}
	message, _ = errObj["message"].(string)
	if rawFields, ok := errObj["fields"].([]any); ok {
		for _, rf := range rawFields {
			if m, ok := rf.(map[string]any); ok {
				f, _ := m["field"].(string)
				msg, _ := m["message"].(string)
				fields = append(fields, ValidationField{Field: f, Message: msg})
			}
		}
	}
	return code, message, fields
}

func asErrorObject(parsed map[string]any) (map[string]any, bool) {
	if parsed == nil {
		return nil, false
	}
	errObj, ok := parsed["error"].(map[string]any)
	return errObj, ok
}
