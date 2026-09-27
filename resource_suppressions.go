package duva

import (
	"context"
	"iter"
	"net/http"
	"net/url"
	"strconv"
)

// SuppressionsService is Client.Suppressions.
type SuppressionsService struct{ cfg *config }

// ListSuppressionsParams filters one page of List / ListAll.
type ListSuppressionsParams struct {
	Reason string
	Limit  int
	Cursor string
}

func (p ListSuppressionsParams) query() url.Values {
	q := url.Values{}
	if p.Reason != "" {
		q.Set("reason", p.Reason)
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
func (s *SuppressionsService) List(ctx context.Context, params ListSuppressionsParams) (*SuppressionPage, error) {
	var page SuppressionPage
	_, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodGet, Path: s.cfg.domainPath() + "/suppressions", Query: params.query(), SafeRetry: true,
	}, &page)
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// ListAll follows next_cursor across every page, never loading them all into memory at once.
// maxItems <= 0 means unlimited.
func (s *SuppressionsService) ListAll(ctx context.Context, params ListSuppressionsParams, maxItems int) iter.Seq2[*Suppression, error] {
	return func(yield func(*Suppression, error) bool) {
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

// Add suppresses an address. created is true on a 201 (newly added), false on a 200 (already
// suppressed, for this reason or another).
func (s *SuppressionsService) Add(ctx context.Context, email string) (suppression *Suppression, created bool, err error) {
	var sup Suppression
	raw, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodPost, Path: s.cfg.domainPath() + "/suppressions", Body: map[string]string{"email": email},
		// NOT safe to retry: the outcome of a timed-out first attempt is unknown.
	}, &sup)
	if err != nil {
		return nil, false, err
	}
	return &sup, raw.Status == http.StatusCreated, nil
}

// Remove un-suppresses an address. A *NotFoundError means it wasn't suppressed.
func (s *SuppressionsService) Remove(ctx context.Context, email string) error {
	_, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodDelete, Path: s.cfg.domainPath() + "/suppressions/" + pathEscape(email),
	}, nil) // NOT safe to retry
	return err
}
