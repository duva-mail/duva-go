// Transport: builds requests, applies the retry policy of docs/bibliotheques-clientes.md section
// 3.5 (in the duva repository) exactly, and turns a Duva error response into the right typed
// error.
package duva

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

type requestSpec struct {
	Method         string
	Path           string
	Query          url.Values
	Body           any
	IdempotencyKey string
	// SafeRetry: whether the WHOLE call may be retried after a network failure or a 5xx (distinct
	// from the per-error-code Retry-After policy, which always applies): false for a write whose
	// outcome, after a timeout, is unknown (see docs/bibliotheques-clientes.md section 3.5).
	SafeRetry bool
}

type rawResponse struct {
	Data   []byte
	Header http.Header
	Status int
}

func buildRequest(ctx context.Context, cfg *config, spec requestSpec) (*http.Request, error) {
	target := cfg.baseURL + spec.Path
	if len(spec.Query) > 0 {
		target += "?" + spec.Query.Encode()
	}
	var body io.Reader
	if spec.Body != nil {
		encoded, err := json.Marshal(spec.Body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, spec.Method, target, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "Bearer "+cfg.apiKey)
	req.Header.Set("user-agent", cfg.userAgentHeader())
	if cfg.language != "" {
		req.Header.Set("accept-language", cfg.language)
	}
	if spec.IdempotencyKey != "" {
		req.Header.Set("idempotency-key", spec.IdempotencyKey)
	}
	if spec.Body != nil {
		req.Header.Set("content-type", "application/json")
	}
	return req, nil
}

func doRequest(ctx context.Context, cfg *config, spec requestSpec) (*rawResponse, error) {
	attempt := 0
	for {
		req, err := buildRequest(ctx, cfg, spec)
		if err != nil {
			return nil, err
		}
		resp, doErr := cfg.httpClient.Do(req)
		var callErr error
		var raw *rawResponse
		if doErr != nil {
			callErr = classifyTransportError(doErr)
		} else {
			raw, callErr = parseResponse(resp)
		}
		if callErr == nil {
			return raw, nil
		}
		wait, retry := retryDelay(callErr, spec, attempt, cfg)
		if !retry {
			return nil, callErr
		}
		attempt++
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// doRequestJSON runs doRequest and decodes a JSON body into out (skipped when out is nil or the
// body is empty, e.g. a 204).
func doRequestJSON(ctx context.Context, cfg *config, spec requestSpec, out any) (*rawResponse, error) {
	raw, err := doRequest(ctx, cfg, spec)
	if err != nil {
		return nil, err
	}
	if out != nil && len(raw.Data) > 0 {
		if err := json.Unmarshal(raw.Data, out); err != nil {
			return nil, &ServerError{newAPIError(
				raw.Status, "http_error", "Duva answered with an unexpected body.", string(raw.Data), nil,
			)}
		}
	}
	return raw, nil
}

func classifyTransportError(err error) error {
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &TimeoutError{Err: err}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &TimeoutError{Err: err}
	}
	return &ConnectionError{Err: err}
}

func parseResponse(resp *http.Response) (*rawResponse, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	if resp.StatusCode == http.StatusNoContent {
		return &rawResponse{Data: nil, Header: resp.Header, Status: resp.StatusCode}, nil
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &rawResponse{Data: data, Header: resp.Header, Status: resp.StatusCode}, nil
	}
	var parsed map[string]any
	if len(data) > 0 {
		_ = json.Unmarshal(data, &parsed) // a malformed body just leaves parsed nil (-> http_error)
	}
	return nil, errorFromResponse(resp.StatusCode, parsed, string(data), resp.Header.Get("retry-after"))
}

func retryDelay(err error, spec requestSpec, attempt int, cfg *config) (time.Duration, bool) {
	var rateLimit *RateLimitError
	if errors.As(err, &rateLimit) {
		if rateLimit.RetryAfter > cfg.maxRetryWait {
			return 0, false
		}
		return rateLimit.RetryAfter, true
	}
	var serverErr *ServerError
	isServerError := errors.As(err, &serverErr)
	if _, ok := err.(hasAPIError); ok && !isServerError {
		// Any other error Duva actually answered (quota exceeded, validation, auth...) is never
		// retried automatically.
		return 0, false
	}
	var connErr *ConnectionError
	var timeoutErr *TimeoutError
	isTransient := isServerError || errors.As(err, &connErr) || errors.As(err, &timeoutErr)
	if !isTransient || !spec.SafeRetry || attempt >= cfg.maxRetries {
		return 0, false
	}
	return backoff(attempt), true
}

// backoff: exponential with jitter, capped: never a fixed delay, never unbounded.
func backoff(attempt int) time.Duration {
	base := math.Min(0.5*math.Pow(2, float64(attempt)), 8.0)
	seconds := base/2 + rand.Float64()*(base/2) // #nosec G404 -- jitter, not cryptographic
	return time.Duration(seconds * float64(time.Second))
}
