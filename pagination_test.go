package duva_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/duva-mail/duva-go"
)

func TestEventsListAllFollowsEveryPageWithoutLoadingThemAllAtOnce(t *testing.T) {
	pages := []map[string]any{
		{"data": []any{map[string]any{"id": "evt_1", "type": "delivered", "message_id": "m", "recipient": "r", "occurred_at": "2026-01-01T00:00:00Z", "detail": map[string]any{}, "metadata": map[string]any{}}}, "next_cursor": "c1"},
		{"data": []any{map[string]any{"id": "evt_2", "type": "delivered", "message_id": "m", "recipient": "r", "occurred_at": "2026-01-01T00:00:00Z", "detail": map[string]any{}, "metadata": map[string]any{}}}, "next_cursor": nil},
	}
	call := 0
	client, _ := fakeClient(t, func(req *http.Request) (*http.Response, error) {
		page := pages[call]
		call++
		return duva.JSONResponse(200, page), nil
	})

	var ids []string
	for event, err := range client.Events.ListAll(context.Background(), duva.ListEventsParams{}, 0) {
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		ids = append(ids, event.Id)
	}
	if len(ids) != 2 || ids[0] != "evt_1" || ids[1] != "evt_2" {
		t.Fatalf("unexpected ids: %v", ids)
	}
	if call != 2 {
		t.Fatalf("expected 2 page requests, got %d", call)
	}
}

func TestEventsListAllStopsAtMaxItems(t *testing.T) {
	page := map[string]any{
		"data": []any{
			map[string]any{"id": "evt_1", "type": "delivered", "message_id": "m", "recipient": "r", "occurred_at": "2026-01-01T00:00:00Z", "detail": map[string]any{}, "metadata": map[string]any{}},
			map[string]any{"id": "evt_2", "type": "delivered", "message_id": "m", "recipient": "r", "occurred_at": "2026-01-01T00:00:00Z", "detail": map[string]any{}, "metadata": map[string]any{}},
			map[string]any{"id": "evt_3", "type": "delivered", "message_id": "m", "recipient": "r", "occurred_at": "2026-01-01T00:00:00Z", "detail": map[string]any{}, "metadata": map[string]any{}},
		},
		"next_cursor": "c1",
	}
	client, _ := fakeClient(t, func(req *http.Request) (*http.Response, error) {
		return duva.JSONResponse(200, page), nil
	})

	var ids []string
	for event, err := range client.Events.ListAll(context.Background(), duva.ListEventsParams{}, 2) {
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		ids = append(ids, event.Id)
	}
	if len(ids) != 2 {
		t.Fatalf("expected exactly 2 items (max_items), got %d: %v", len(ids), ids)
	}
}
