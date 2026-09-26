package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// --- Serply Search Provider ---
//
// Serply returns Google web results as JSON on a tenant-supplied key. Requests
// are GET with the key in an X-Api-Key header; results arrive under "results"
// with title/link/description fields. The API caps a response at 10 results,
// which is already maxSearchCount here, so no extra clamping is needed.

type serplySearchProvider struct {
	apiKey     string
	maxResults int
	client     *http.Client
	endpoint   string
}

func newSerplySearchProvider(apiKey string, maxResults int) *serplySearchProvider {
	return &serplySearchProvider{
		apiKey:     apiKey,
		maxResults: normalizeProviderMaxResults(maxResults),
		client:     &http.Client{Timeout: time.Duration(searchTimeoutSeconds) * time.Second},
		endpoint:   serplySearchEndpoint,
	}
}

func (p *serplySearchProvider) Name() string { return searchProviderSerply }

// serplyFreshnessTBS maps the tool's freshness shortcuts onto Google's tbs
// recency filter, which Serply forwards upstream unchanged.
//
// normalizeFreshness also accepts a YYYY-MM-DDtoYYYY-MM-DD range. That form is
// deliberately absent: upstream ignores the equivalent cdr range and answers
// with unfiltered results, so sending it would report a date filter the
// results do not actually honour. A range therefore leaves tbs unset.
var serplyFreshnessTBS = map[string]string{
	"pd": "qdr:d",
	"pw": "qdr:w",
	"pm": "qdr:m",
	"py": "qdr:y",
}

func (p *serplySearchProvider) Search(ctx context.Context, params searchParams) ([]searchResult, error) {
	limit := clampProviderResultCount(params.Count, p.maxResults)

	q := url.Values{}
	q.Set("q", params.Query)
	q.Set("num", strconv.Itoa(limit))

	if params.Country != "" {
		q.Set("gl", params.Country)
	}
	if lang := coalesceSearchText(params.SearchLang, params.UILang); lang != "" {
		q.Set("hl", lang)
	}
	if tbs, ok := serplyFreshnessTBS[normalizeFreshness(params.Freshness)]; ok {
		q.Set("tbs", tbs)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", p.apiKey)
	// Serply attributes usage by client, so identify the gateway rather than
	// sending the browser webSearchUserAgent the scraping providers need.
	req.Header.Set("User-Agent", "goclaw")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serply API returned %d: %s", resp.StatusCode, truncateStr(string(body), 200))
	}

	var serplyResp struct {
		Results []struct {
			Title       string `json:"title"`
			Link        string `json:"link"`
			Description string `json:"description"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &serplyResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	results := make([]searchResult, 0, len(serplyResp.Results))
	for _, r := range serplyResp.Results {
		if len(results) >= limit {
			break
		}
		link := strings.TrimSpace(r.Link)
		if link == "" {
			continue
		}
		results = append(results, searchResult{
			Title:       coalesceSearchText(r.Title, link, "Untitled"),
			URL:         link,
			Description: truncateStr(strings.TrimSpace(r.Description), 240),
		})
	}
	return results, nil
}
