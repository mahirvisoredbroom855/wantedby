// HN Algolia adapter — fetches Ask HN and Show HN posts matching frustration phrases.
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	hnSearchURL = "https://hn.algolia.com/api/v1/search_by_date"
	hnUserAgent = "wantedby-ingestor/1.0"
)

// HNAdapter fetches frustration-signal posts from Hacker News via the Algolia search API.
// No authentication required.
type HNAdapter struct {
	httpClient *http.Client
	logger     *slog.Logger
}

// NewHNAdapter constructs an HNAdapter.
func NewHNAdapter(logger *slog.Logger) *HNAdapter {
	return &HNAdapter{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		logger:     logger,
	}
}

// Name returns the platform identifier.
func (h *HNAdapter) Name() string { return "hn" }

// FetchSince returns all Ask HN / Show HN posts and comments published after since
// that match any frustration-signal phrase.
func (h *HNAdapter) FetchSince(ctx context.Context, since time.Time) ([]RawPost, error) {
	sinceUnix := since.Unix()
	var all []RawPost

	for _, phrase := range frustrationPhrases {
		posts, err := h.searchPhrase(ctx, phrase, sinceUnix)
		if err != nil {
			h.logger.Warn("hn search failed", "phrase", phrase, "error", err)
			continue
		}
		all = append(all, posts...)
	}
	return all, nil
}

// searchPhrase queries Algolia for one phrase across Ask HN and Show HN.
func (h *HNAdapter) searchPhrase(ctx context.Context, phrase string, sinceUnix int64) ([]RawPost, error) {
	params := url.Values{
		"query":          {phrase},
		"tags":           {"(ask_hn,show_hn)"},
		"hitsPerPage":    {"100"},
		"numericFilters": {fmt.Sprintf("created_at_i>%d", sinceUnix)},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hnSearchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", hnUserAgent)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("algolia status %d: %s", resp.StatusCode, string(body))
	}

	var result hnSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	fetchedAt := time.Now().UTC()
	var posts []RawPost
	for _, hit := range result.Hits {
		createdAt := time.Unix(hit.CreatedAtI, 0).UTC()

		title := hit.Title
		if title == "" {
			title = hit.StoryTitle
		}
		body := hit.Text
		if body == "" {
			body = hit.Comment
		}

		raw := map[string]any{
			"objectID":    hit.ObjectID,
			"story_id":    hit.StoryID,
			"title":       title,
			"text":        body,
			"author":      hit.Author,
			"url":         hit.URL,
			"points":      hit.Points,
			"num_comments": hit.NumComments,
			"created_at_i": hit.CreatedAtI,
			"_tags":       hit.Tags,
		}

		posts = append(posts, RawPost{
			PlatformID:   "hn:" + hit.ObjectID,
			Platform:     "hn",
			Title:        title,
			Body:         body,
			Author:       hit.Author,
			URL:          fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID),
			Subreddit:    "",
			Score:        hit.Points,
			CommentCount: hit.NumComments,
			CreatedAt:    createdAt,
			FetchedAt:    fetchedAt,
			Raw:          raw,
		})
	}
	return posts, nil
}

// --- Algolia HN response types ---

type hnSearchResult struct {
	Hits []hnHit `json:"hits"`
}

type hnHit struct {
	ObjectID    string   `json:"objectID"`
	StoryID     int64    `json:"story_id"`
	Title       string   `json:"title"`
	StoryTitle  string   `json:"story_title"`
	Text        string   `json:"text"`
	Comment     string   `json:"comment_text"`
	Author      string   `json:"author"`
	URL         string   `json:"url"`
	Points      int      `json:"points"`
	NumComments int      `json:"num_comments"`
	CreatedAtI  int64    `json:"created_at_i"`
	Tags        []string `json:"_tags"`
}
