package duva

import "github.com/duva-mail/duva-go/internal/generated"

// Response types, generated from docs/openapi.json (internal/generated, never edited by hand;
// see scripts/generate.sh). Re-exported here so callers only ever import this package.
type (
	Message               = generated.Message
	MessageStatus         = generated.MessageStatus
	MessageAccepted       = generated.MessageAccepted
	MessageAcceptedStatus = generated.MessageAcceptedStatus
	Recipient             = generated.Recipient
	RecipientStatus       = generated.RecipientStatus
	Event                 = generated.Event
	EventType             = generated.EventType
	EventPage             = generated.EventPage
	Suppression           = generated.Suppression
	SuppressionReason     = generated.SuppressionReason
	SuppressionPage       = generated.SuppressionPage
	Webhook               = generated.Webhook
	WebhookStatus         = generated.WebhookStatus
	WebhookDelivery       = generated.WebhookDelivery
	WebhookDeliveryStatus = generated.WebhookDeliveryStatus
	WebhookDeliveryList   = generated.WebhookDeliveryList
	WebhookList           = generated.WebhookList
	Stats                 = generated.Stats
	StatsPeriod           = generated.StatsPeriod
	StatsGranularity      = generated.StatsGranularity
	Tracking              = generated.Tracking
	// Attachment is both the wire format Messages.Send sends and what NewAttachmentFromBytes /
	// NewAttachmentFromFile build; see attachment.go.
	Attachment = generated.Attachment
)

// SendMessageResult is what Messages.Send returns: the accepted message id and status, plus what
// the response headers say about idempotency replay and the new message's location.
type SendMessageResult struct {
	ID     string
	Status string
	// Replayed is true when this answer replays an earlier identical request (the
	// Idempotent-Replayed response header): a retried call with the same idempotency key never
	// creates a duplicate.
	Replayed bool
	// Location is GET /v1/{domain}/messages/{id}'s address (the Location response header), or
	// empty if absent.
	Location string
}

// HealthStatus is what Health returns.
type HealthStatus struct {
	Status string `json:"status"`
}
