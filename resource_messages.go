package duva

import (
	"context"
	"net/http"
)

// MessagesService is Client.Messages.
type MessagesService struct{ cfg *config }

// SendMessageParams: From, To and Subject are required; at least one of HTML and Text too.
// Nothing more is checked locally: recopying the server's own rules would drift, and it answers
// 422 with Fields regardless.
type SendMessageParams struct {
	From        string
	To          []string
	Subject     string
	HTML        string
	Text        string
	Tags        []string
	Tracking    *Tracking
	ReplyTo     string
	Headers     map[string]string
	Metadata    map[string]string
	Attachments []Attachment
	// IdempotencyKey: generated (UUID v4) when empty. 1 to 255 printable ASCII characters, no
	// spaces, unique per domain, when you supply your own. A key generated per call does not
	// de-duplicate BETWEEN two calls: to de-duplicate a logical send (an order), pass a stable key
	// ("order-4821").
	IdempotencyKey string
}

// Send sends a message: always asynchronous ("queued"), never returns a delivery outcome.
func (s *MessagesService) Send(ctx context.Context, params SendMessageParams) (*SendMessageResult, error) {
	if err := assertAttachmentLimits(params.Attachments); err != nil {
		return nil, err
	}
	idempotencyKey := params.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = newIdempotencyKey()
	} else if err := validateIdempotencyKey(idempotencyKey); err != nil {
		return nil, err
	}

	body := map[string]any{"from": params.From, "to": params.To, "subject": params.Subject}
	if params.HTML != "" {
		body["html"] = params.HTML
	}
	if params.Text != "" {
		body["text"] = params.Text
	}
	if params.Tags != nil {
		body["tags"] = params.Tags
	}
	if params.Tracking != nil {
		body["tracking"] = params.Tracking
	}
	if params.ReplyTo != "" {
		body["reply_to"] = params.ReplyTo
	}
	if params.Headers != nil {
		body["headers"] = params.Headers
	}
	if params.Metadata != nil {
		body["metadata"] = params.Metadata
	}
	if params.Attachments != nil {
		body["attachments"] = params.Attachments
	}

	var accepted MessageAccepted
	raw, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method:         http.MethodPost,
		Path:           s.cfg.domainPath() + "/messages",
		Body:           body,
		IdempotencyKey: idempotencyKey,
		SafeRetry:      true, // protected by the idempotency key
	}, &accepted)
	if err != nil {
		return nil, err
	}
	return &SendMessageResult{
		ID:       accepted.Id,
		Status:   string(accepted.Status),
		Replayed: raw.Header.Get("Idempotent-Replayed") == "true",
		Location: raw.Header.Get("Location"),
	}, nil
}

// Get reads a message, with its recipients.
func (s *MessagesService) Get(ctx context.Context, id string) (*Message, error) {
	var msg Message
	_, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodGet, Path: s.cfg.domainPath() + "/messages/" + pathEscape(id), SafeRetry: true,
	}, &msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}
