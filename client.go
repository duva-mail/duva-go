package duva

import (
	"context"
	"net/http"
)

// Client is bound to one domain and its API key (an API key only ever opens one domain).
//
//	client, err := duva.New("dv_...", "example.com") // or DUVA_API_KEY / DUVA_DOMAIN
type Client struct {
	cfg *config

	Messages     *MessagesService
	Events       *EventsService
	Suppressions *SuppressionsService
	Webhooks     *WebhooksService
	Stats        *StatsService
}

// New builds a Client. apiKey and domain fall back to DUVA_API_KEY / DUVA_DOMAIN when empty.
func New(apiKey, domain string, opts ...Option) (*Client, error) {
	cfg, err := resolveConfig(apiKey, domain, opts)
	if err != nil {
		return nil, err
	}
	return &Client{
		cfg:          cfg,
		Messages:     &MessagesService{cfg: cfg},
		Events:       &EventsService{cfg: cfg},
		Suppressions: &SuppressionsService{cfg: cfg},
		Webhooks:     &WebhooksService{cfg: cfg},
		Stats:        &StatsService{cfg: cfg},
	}, nil
}

// Health checks Duva's own health (GET /health). A 503 or unreachable API returns an error.
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	var out HealthStatus
	_, err := doRequestJSON(ctx, c.cfg, requestSpec{Method: http.MethodGet, Path: "/health", SafeRetry: true}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
