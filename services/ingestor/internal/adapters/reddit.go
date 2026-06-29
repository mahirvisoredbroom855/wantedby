// Package adapters — Reddit OAuth2 adapter.
// Fetches posts and comments from 12 target subreddits matching frustration-signal phrases.
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
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	redditTokenURL   = "https://www.reddit.com/api/v1/access_token"
	redditSearchURL  = "https://oauth.reddit.com/r/%s/search.json"
	redditUserAgent  = "wantedby-ingestor/1.0 (by /u/wantedby_bot)"
	redditMaxWorkers = 5
)

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

// RedditAdapter fetches frustration-signal posts from Reddit via OAuth2.
type RedditAdapter struct {
	clientID     string
	clientSecret string
	limiter      *rate.Limiter
	httpClient   *http.Client
	logger       *slog.Logger

	tokenMu     sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// NewRedditAdapter constructs a RedditAdapter. clientID and clientSecret come from env vars.
func NewRedditAdapter(clientID, clientSecret string, logger *slog.Logger) *RedditAdapter {
	return &RedditAdapter{
		clientID:     clientID,
		clientSecret: clientSecret,
		limiter:      rate.NewLimiter(rate.Every(time.Minute/60), 1),
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		logger:       logger,
	}
}

// Name returns the platform identifier.
func (r *RedditAdapter) Name() string { return "reddit" }

// FetchSince returns all matching posts from all 12 subreddits published after since.
// Subreddit fetches are parallelised with a worker pool of max 5.
func (r *RedditAdapter) FetchSince(ctx context.Context, since time.Time) ([]RawPost, error) {
	if err := r.ensureToken(ctx); err != nil {
		return nil, fmt.Errorf("reddit auth: %w", err)
	}

	type result struct {
		posts []RawPost
		err   error
	}

	jobs := make(chan string, len(targetSubreddits))
	results := make(chan result, len(targetSubreddits))

	for i := 0; i < redditMaxWorkers; i++ {
		go func() {
			for sub := range jobs {
				posts, err := r.fetchSubreddit(ctx, sub, since)
				results <- result{posts: posts, err: err}
			}
		}()
	}

	for _, sub := range targetSubreddits {
		jobs <- sub
	}
	close(jobs)

	var all []RawPost
	for range targetSubreddits {
		res := <-results
		if res.err != nil {
			r.logger.Error("reddit fetch error", "error", res.err)
			continue
		}
		all = append(all, res.posts...)
	}
	return all, nil
}

// fetchSubreddit searches one subreddit for all frustration phrases published after since.
func (r *RedditAdapter) fetchSubreddit(ctx context.Context, subreddit string, since time.Time) ([]RawPost, error) {
	var posts []RawPost
	sinceUnix := since.Unix()

	for _, phrase := range frustrationPhrases {
		if err := r.limiter.Wait(ctx); err != nil {
			return posts, fmt.Errorf("rate limiter: %w", err)
		}

		batch, err := r.searchSubreddit(ctx, subreddit, phrase, sinceUnix)
		if err != nil {
			r.logger.Warn("subreddit search failed", "subreddit", subreddit, "phrase", phrase, "error", err)
			continue
		}
		posts = append(posts, batch...)
	}
	return posts, nil
}

// searchSubreddit executes one search query against the Reddit API.
func (r *RedditAdapter) searchSubreddit(ctx context.Context, subreddit, query string, sinceUnix int64) ([]RawPost, error) {
	endpoint := fmt.Sprintf(redditSearchURL, subreddit)
	params := url.Values{
		"q":       {query},
		"sort":    {"new"},
		"limit":   {"100"},
		"type":    {"link"},
		"t":       {"month"},
		"restrict_sr": {"true"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.accessToken)
	req.Header.Set("User-Agent", redditUserAgent)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("reddit api status %d: %s", resp.StatusCode, string(body))
	}

	var listing redditListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, fmt.Errorf("decode listing: %w", err)
	}

	var posts []RawPost
	fetchedAt := time.Now().UTC()
	for _, child := range listing.Data.Children {
		p := child.Data
		createdAt := time.Unix(int64(p.CreatedUTC), 0).UTC()
		if createdAt.Unix() <= sinceUnix {
			continue
		}

		raw := map[string]any{
			"id":            p.ID,
			"name":          p.Name,
			"subreddit":     p.Subreddit,
			"title":         p.Title,
			"selftext":      p.Selftext,
			"author":        p.Author,
			"url":           p.URL,
			"permalink":     p.Permalink,
			"score":         p.Score,
			"num_comments":  p.NumComments,
			"created_utc":   p.CreatedUTC,
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

// ensureToken obtains or refreshes the OAuth2 access token using client credentials flow.
func (r *RedditAdapter) ensureToken(ctx context.Context) error {
	r.tokenMu.Lock()
	defer r.tokenMu.Unlock()

	if r.accessToken != "" && time.Now().Before(r.tokenExpiry.Add(-30*time.Second)) {
		return nil
	}

	body := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, redditTokenURL, body)
	if err != nil {
		return fmt.Errorf("build token request: %w", err)
	}
	req.SetBasicAuth(r.clientID, r.clientSecret)
	req.Header.Set("User-Agent", redditUserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token endpoint status %d: %s", resp.StatusCode, string(raw))
	}

	var tok redditToken
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return fmt.Errorf("decode token: %w", err)
	}

	r.accessToken = tok.AccessToken
	r.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	r.logger.Info("reddit token refreshed", "expires_in_seconds", tok.ExpiresIn)
	return nil
}

// --- Reddit API response types ---

type redditToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type redditListing struct {
	Data struct {
		Children []struct {
			Data redditPost `json:"data"`
		} `json:"children"`
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
