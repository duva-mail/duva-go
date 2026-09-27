package duva

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// StatsService is Client.Stats.
type StatsService struct{ cfg *config }

// GetStatsParams: Granularity is "day" (default) or "hour"; periods are in UTC.
type GetStatsParams struct {
	Granularity string
	Since       *time.Time
	Until       *time.Time
}

// Get reads aggregated send statistics for this domain.
func (s *StatsService) Get(ctx context.Context, params GetStatsParams) (*Stats, error) {
	q := url.Values{}
	if params.Granularity != "" {
		q.Set("granularity", params.Granularity)
	}
	if params.Since != nil {
		q.Set("since", params.Since.UTC().Format(time.RFC3339Nano))
	}
	if params.Until != nil {
		q.Set("until", params.Until.UTC().Format(time.RFC3339Nano))
	}
	var stats Stats
	_, err := doRequestJSON(ctx, s.cfg, requestSpec{
		Method: http.MethodGet, Path: s.cfg.domainPath() + "/stats", Query: q, SafeRetry: true,
	}, &stats)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}
