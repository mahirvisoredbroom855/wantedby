// Package adapters — Reddit adapter via RapidAPI's "Reddit34" wrapper.
//
// Reddit's official OAuth2 Data API now requires manual approval ("valid moderation
// use case") that this project's read-only analytics use case does not qualify for
// under self-service. This adapter is a stopgap against a commercial third-party API
// while the official Reddit Data Access Request is pending. Swapping back to OAuth2
// later only requires rewriting this file — nothing outside it depends on the
// transport mechanism, by design of the PlatformAdapter interface.
//
// Fetches each subreddit's newest posts once per call (not once per search phrase —
// metered APIs charge per request, so phrase-matching happens client-side instead).
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const redditRapidAPIPathFmt = "https://%s/getPostsBySubreddit"

var targetSubreddits = []string{
	"SideProject",
	"IndieHackers",
	"Entrepreneur",
	"webdev",
	"programming",
	"startups",
	"smallbusiness",
	"freelance",
	"devops",
	"MachineLearning",
	"LocalLLaMA",
	"selfhosted",
}

var frustrationPhrases = []string{
	"I wish there was a tool that",
	"why doesn't something exist",
	"I can't believe there's no",
	"I've been looking for something that",
	"does anyone know a tool that",
	"I had to do this manually because",
	"I'm so frustrated that",
	"why is there no app for",
	"I ended up building my own because",
	"the existing solutions are terrible",
	"I would pay for something that",
	"has anyone built something that",
	"I keep Googling for",
	"every time I need to",
}

// redditRequestDelay throttles requests to one at a time with a pause between
// each call. RapidAPI's Basic/free tier rate-limits well below its advertised
// hourly ceiling on request bursts, so concurrency buys nothing here and only
// risks 429s — sequential is both safe and sufficient at this volume.
// A var (not const) so tests can shrink it and avoid a slow test suite.
var redditRequestDelay = 2 * time.Second

// RedditAdapter fetches subreddit posts via the RapidAPI Reddit34 wrapper and
// filters for frustration-signal phrases client-side.
type RedditAdapter struct {
	apiKey     string
	apiHost    string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewRedditAdapter constructs a RedditAdapter. apiKey and apiHost come from
// REDDIT_RAPIDAPI_KEY and REDDIT_RAPIDAPI_HOST env vars.
func NewRedditAdapter(apiKey, apiHost string, logger *slog.Logger) *RedditAdapter {
	return &RedditAdapter{
		apiKey:     apiKey,
		apiHost:    apiHost,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		logger:     logger,
	}
}

// Name returns the platform identifier.
func (r *RedditAdapter) Name() string { return "reddit" }

// FetchSince fetches the newest posts from all 12 target subreddits, one
// request at a time with a delay between each, and returns only those
// published after since AND matching at least one frustration-signal phrase
// in title or body.
func (r *RedditAdapter) FetchSince(ctx context.Context, since time.Time) ([]RawPost, error) {
	var all []RawPost

	for i, sub := range targetSubreddits {
		posts, err := r.fetchSubreddit(ctx, sub, since)
		if err != nil {
			r.logger.Error("reddit fetch error", "subreddit", sub, "error", err)
			continue
		}
		all = append(all, posts...)

		if i < len(targetSubreddits)-1 {
			select {
			case <-ctx.Done():
				return all, ctx.Err()
			case <-time.After(redditRequestDelay):
			}
		}
	}
	return all, nil
}

// fetchSubreddit makes one API call for the newest posts in a subreddit, then
// filters client-side for frustration phrases and the since timestamp.
func (r *RedditAdapter) fetchSubreddit(ctx context.Context, subreddit string, since time.Time) ([]RawPost, error) {
	endpoint := fmt.Sprintf(redditRapidAPIPathFmt, r.apiHost)
	params := url.Values{
		"subreddit": {subreddit},
		"sort":      {"new"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-rapidapi-key", r.apiKey)
	req.Header.Set("x-rapidapi-host", r.apiHost)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("rapidapi status %d: %s", resp.StatusCode, string(body))
	}

	var result rapidAPIPostsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("rapidapi reported failure for subreddit %s", subreddit)
	}

	sinceUnix := since.Unix()
	fetchedAt := time.Now().UTC()
	var posts []RawPost

	for _, child := range result.Data.Posts {
		p := child.Data
		createdAt := time.Unix(int64(p.CreatedUTC), 0).UTC()
		if createdAt.Unix() <= sinceUnix {
			continue
		}
		if !matchesFrustrationPhrase(p.Title, p.Selftext) {
			continue
		}

		raw := map[string]any{
			"id":           p.ID,
			"name":         p.Name,
			"subreddit":    p.Subreddit,
			"title":        p.Title,
			"selftext":     p.Selftext,
			"author":       p.Author,
			"url":          p.URL,
			"permalink":    p.Permalink,
			"score":        p.Score,
			"num_comments": p.NumComments,
			"created_utc":  p.CreatedUTC,
		}

		posts = append(posts, RawPost{
			PlatformID:   "reddit:" + p.Name,
			Platform:     "reddit",
			Title:        p.Title,
			Body:         p.Selftext,
			Author:       p.Author,
			URL:          "https://reddit.com" + p.Permalink,
			Subreddit:    p.Subreddit,
			Score:        p.Score,
			CommentCount: p.NumComments,
			CreatedAt:    createdAt,
			FetchedAt:    fetchedAt,
			Raw:          raw,
		})
	}
	return posts, nil
}

// matchesFrustrationPhrase reports whether the title or body contains any of the
// configured frustration-signal phrases, case-insensitive.
func matchesFrustrationPhrase(title, body string) bool {
	haystack := strings.ToLower(title + " " + body)
	for _, phrase := range frustrationPhrases {
		if strings.Contains(haystack, strings.ToLower(phrase)) {
			return true
		}
	}
	return false
}

// --- RapidAPI (Reddit34) response types ---
// Shape: { success, data: { cursor, posts: [ { kind, data: redditPost } ] } }
// The inner "data" object matches Reddit's own native listing schema field-for-field.

type rapidAPIPostsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Cursor string `json:"cursor"`
		Posts  []struct {
			Kind string     `json:"kind"`
			Data redditPost `json:"data"`
		} `json:"posts"`
	} `json:"data"`
}

type redditPost struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"` // fullname e.g. t3_abc123
	Subreddit   string  `json:"subreddit"`
	Title       string  `json:"title"`
	Selftext    string  `json:"selftext"`
	Author      string  `json:"author"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	CreatedUTC  float64 `json:"created_utc"`
}
