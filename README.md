# Duva for Go

The official [Duva](https://duva.ca) client library for Go. Duva is a transactional email API
hosted in Canada.

```bash
go get github.com/duva-mail/duva-go
```

Requires Go 1.23 or later (`iter.Seq2` range-over-func pagination).

## Sending a message

```go
package main

import (
	"context"
	"os"

	"github.com/duva-mail/duva-go"
)

func main() {
	client, err := duva.New(os.Getenv("DUVA_API_KEY"), "example.com") // or New("dv_...", "example.com")
	if err != nil {
		panic(err)
	}

	message, err := client.Messages.Send(context.Background(), duva.SendMessageParams{
		From:    "Example <notifications@example.com>",
		To:      []string{"client@example.org"},
		Subject: "Your order",
		Text:    "Thank you for your order.",
	})
	if err != nil {
		panic(err)
	}
	println(message.ID, message.Status) // "queued": always asynchronous
}
```

## Reading events and pagination

```go
for event, err := range client.Events.ListAll(ctx, duva.ListEventsParams{Type: "bounced"}, 0) {
	if err != nil {
		panic(err)
	}
	println(event.Type, event.Detail["recipient"])
}
```

`Events.List` / `Suppressions.List` return one page (`.Data`, `.NextCursor`); `Events.ListAll` /
`Suppressions.ListAll` return an `iter.Seq2[*T, error]` that follows `NextCursor` for you, without
ever loading every page into memory, bounded by an optional `maxItems` (0 = unlimited).

## Verifying a webhook

```go
event, err := duva.ConstructEvent([]string{os.Getenv("DUVA_WEBHOOK_SECRET")}, r.Header, rawBody, nil)
if err != nil {
	// respond 400
	return
}
println(event.Type, event.Data["message_id"])
```

`rawBody` must be the **exact bytes** Duva sent (read the request body directly; never a
re-serialized `json.Marshal` of a parsed struct): re-encoding it changes the bytes and invalidates
the signature. Rotating your webhook secret? Pass both — `ConstructEvent([]string{old, new}, ...)`
— while both are active.

## Errors

Every error Duva answers with is one of this package's typed errors; use `errors.As` for the type
you expect, or `duva.ErrorCode(err)` for the code (the contract — never the message, which can
change):

```go
result, err := client.Messages.Send(ctx, params)
var validationErr *duva.ValidationError
var quotaErr *duva.QuotaExceededError
var notFoundErr *duva.NotFoundError
switch {
case errors.As(err, &validationErr):
	fmt.Println(validationErr.Fields) // [{Field: "to[0]", Message: "..."}]
case errors.As(err, &quotaErr):
	fmt.Printf("retry in %s\n", quotaErr.RetryAfter)
case errors.As(err, &notFoundErr):
	// the API key, domain or resource could not be found
}
```

Network failures and timeouts return `*duva.ConnectionError` / `*duva.TimeoutError` instead (no
HTTP response was ever received). Reads and `Messages.Send` (idempotency-key protected) are
retried automatically on a transient failure; `Suppressions.Add`/`Remove` and
`Webhooks.Create`/`Delete` are not, because the outcome of a timed-out first attempt is unknown. A
`429 quota_exceeded` is never retried automatically (its `RetryAfter` can be hours); a
`429 rate_limited` is, as long as the wait fits within `WithMaxRetryWait` (30s by default).

## Attachments

```go
attachment, err := duva.NewAttachmentFromFile("./invoice.pdf")
if err != nil {
	panic(err)
}
params.Attachments = []duva.Attachment{attachment}
```

`NewAttachmentFromBytes(filename, content, opts...)` works from bytes already in memory;
`duva.WithContentID(id)` turns the attachment into an inline image the HTML references with
`cid:`.

## Testing your own application

```go
transport := duva.NewTestTransport(func(req *http.Request) (*http.Response, error) {
	return duva.JSONResponse(200, map[string]any{"status": "ok"}), nil
})
client, _ := duva.New("dv_test", "example.com", duva.WithHTTPClient(transport))
client.Health(ctx)
transport.Requests[0].Path // "/health"
```

`duva.TestTransport` implements `Doer` and records every `RecordedRequest` it receives: no
network call, no separate HTTP mocking library required.

## Configuration

`New(apiKey, domain string, opts ...Option)`, with functional options:

| Option | Default | |
|---|---|---|
| `apiKey` / `domain` args | `DUVA_API_KEY` / `DUVA_DOMAIN` | Required (args win over the environment). |
| `WithBaseURL` | `https://api.duva.ca` | https only (`http://localhost` is allowed for tests). |
| `WithTimeout` | 10s | Only used when no `WithHTTPClient` is given. |
| `WithMaxRetries` | 2 | Network failures / `5xx` on a safe-to-retry call. |
| `WithMaxRetryWait` | 30s | A `429 rate_limited` with a longer wait is not retried. |
| `WithLanguage` | unset | `"en"` or `"fr"`: the language of `error.message`. |
| `WithUserAgent` | unset | Appended to (never replaces) the `User-Agent` header. |
| `WithHTTPClient` | `&http.Client{Timeout: ...}` | Inject any `Doer` (a different `*http.Client`, a proxy, `TestTransport`). |

Zero third-party dependencies: `net/http` only.

## Full reference

The complete API surface and the OpenAPI specification this library follows:
<https://duva.ca/en/docs> and <https://duva.ca/openapi.json>.

## Development

```bash
go build ./...
go vet ./...
bash scripts/generate.sh          # regenerate internal/generated/types.go from the OpenAPI spec
bash scripts/fetch-conformance.sh && go test ./...
```

Response types are generated from `docs/openapi.json` (`internal/generated/`, via
[`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen), never edited by hand) and
re-exported from the package root (`models.go`) so callers only ever import `duva`. The client
itself (retries, pagination, errors, webhooks) is hand-written and checked against the shared
fixtures published in [`duva-mail/duva-conformance`](https://github.com/duva-mail/duva-conformance).

## License

MIT, see [LICENSE](./LICENSE).
