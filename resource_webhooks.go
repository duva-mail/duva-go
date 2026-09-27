package duva

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// WebhooksService is Client.Webhooks.
type WebhooksService struct{ cfg *config }

// CreateWebhookParams: URL is required.
type CreateWebhookParams struct {
	URL    string
	Events []string
}

// Create registers a webhook endpoint. The returned Webhook's Secret is present ONLY in this
// response: it never comes back from List or Get. Never logged, never retried (the outcome of a
// timed-out first attempt is unknown).
func (s *WebhooksService) Create(ctx context.Context, params CreateWebhookParams) (*Webhook, error) {
	body := map[string]any{"url": params.URL}
	if params.Events != nil {
		body["events"] = params.Events
	}
	var wh Webhook
	_, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodPost, Path: s.cfg.domainPath() + "/webhooks", Body: body,
	}, &wh)
	if err != nil {
		return nil, err
	}
	return &wh, nil
}

// List returns every webhook endpoint for this domain.
func (s *WebhooksService) List(ctx context.Context) ([]Webhook, error) {
	var list WebhookList
	_, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodGet, Path: s.cfg.domainPath() + "/webhooks", SafeRetry: true,
	}, &list)
	if err != nil {
		return nil, err
	}
	return list.Data, nil
}

// Get reads one webhook endpoint.
func (s *WebhooksService) Get(ctx context.Context, id string) (*Webhook, error) {
	var wh Webhook
	_, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodGet, Path: s.cfg.domainPath() + "/webhooks/" + pathEscape(id), SafeRetry: true,
	}, &wh)
	if err != nil {
		return nil, err
	}
	return &wh, nil
}

// Delete removes a webhook endpoint. Not safe to retry.
func (s *WebhooksService) Delete(ctx context.Context, id string) error {
	_, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodDelete, Path: s.cfg.domainPath() + "/webhooks/" + pathEscape(id),
	}, nil)
	return err
}

// Deliveries lists this webhook's delivery attempts (most recent first). limit <= 0 uses the
// server's default page size.
func (s *WebhooksService) Deliveries(ctx context.Context, id string, limit int) ([]WebhookDelivery, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var list WebhookDeliveryList
	_, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodGet, Path: s.cfg.domainPath() + "/webhooks/" + pathEscape(id) + "/deliveries",
		Query: q, SafeRetry: true,
	}, &list)
	if err != nil {
		return nil, err
	}
	return list.Data, nil
}
