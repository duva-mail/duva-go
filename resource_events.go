package duva

import (
	"context"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// EventsService is Client.Events.
type EventsService struct{ cfg *config }

// ListEventsParams filters one page of List / ListAll.
type ListEventsParams struct {
	MessageID string
	Type      string
	Recipient string
	Since     *time.Time
	Limit     int
	Cursor    string
}

func (p ListEventsParams) query() url.Values {
	q := url.Values{}
	if p.MessageID != "" {
		q.Set("message_id", p.MessageID)
	}
	if p.Type != "" {
		q.Set("type", p.Type)
	}
	if p.Recipient != "" {
		q.Set("recipient", p.Recipient)
	}
	if p.Since != nil {
		q.Set("since", p.Since.UTC().Format(time.RFC3339Nano))
	}
	if p.Limit != 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	return q
}

// List reads one page.
func (s *EventsService) List(ctx context.Context, params ListEventsParams) (*EventPage, error) {
	var page EventPage
	_, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodGet, Path: s.cfg.domainPath() + "/events", Query: params.query(), SafeRetry: true,
	}, &page)
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// ListAll follows next_cursor across every page, never loading them all into memory at once.
// maxItems <= 0 means unlimited. The cursor stays opaque: never built or interpreted here.
//
//	for event, err := range client.Events.ListAll(ctx, duva.ListEventsParams{}, 0) {
//		if err != nil { ... }
//	}
func (s *EventsService) ListAll(ctx context.Context, params ListEventsParams, maxItems int) iter.Seq2[*Event, error] {
	return func(yield func(*Event, error) bool) {
		count := 0
		for {
			page, err := s.List(ctx, params)
			if err != nil {
				yield(nil, err)
				return
			}
			for i := range page.Data {
				if maxItems > 0 && count >= maxItems {
					return
				}
				if !yield(&page.Data[i], nil) {
					return
				}
				count++
			}
			if page.NextCursor == nil {
				return
			}
			params.Cursor = *page.NextCursor
		}
	}
}
