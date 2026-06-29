package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func TestRedditAdapter_Name(t *testing.T) {
	a := NewRedditAdapter("key", "host", testLogger())
	assert.Equal(t, "reddit", a.Name())
}

func TestMatchesFrustrationPhrase_TitleMatch(t *testing.T) {
	matched := matchesFrustrationPhrase("I wish there was a tool that tracked invoices", "")
	assert.True(t, matched)
}

func TestMatchesFrustrationPhrase_BodyMatch(t *testing.T) {
	matched := matchesFrustrationPhrase("Random title", "the existing solutions are terrible for this")
	assert.True(t, matched)
}

func TestMatchesFrustrationPhrase_CaseInsensitive(t *testing.T) {
	matched := matchesFrustrationPhrase("WHY DOESN'T SOMETHING EXIST for this", "")
	assert.True(t, matched)
}

func TestMatchesFrustrationPhrase_NoMatch(t *testing.T) {
	matched := matchesFrustrationPhrase("Just a regular meme post", "nothing interesting here")
	assert.False(t, matched)
}

// rewriteHostTransport redirects every request to a fixed target base URL,
// regardless of the scheme/host the request was built with — used so the
// adapter's hardcoded RapidAPI URL format can be pointed at an httptest server.
type rewriteHostTransport struct {
	target *url.URL
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	req.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func TestRedditAdapter_FetchSubreddit_ParsesAndFiltersCorrectly(t *testing.T) {
	matchingCreatedUTC := float64(time.Now().Add(-5 * time.Minute).Unix())
	oldCreatedUTC := float64(time.Now().Add(-2 * time.Hour).Unix())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := rapidAPIPostsResponse{Success: true}
		resp.Data.Cursor = "t3_abc"
		resp.Data.Posts = []struct {
			Kind string     `json:"kind"`
			Data redditPost `json:"data"`
		}{
			{
				Kind: "t3",
				Data: redditPost{
					ID:          "abc123",
					Name:        "t3_abc123",
					Subreddit:   "SideProject",
					Title:       "I wish there was a tool that tracks invoice payments",
					Selftext:    "Body text here",
					Author:      "user1",
					Permalink:   "/r/SideProject/comments/abc123",
					Score:       42,
					NumComments: 7,
					CreatedUTC:  matchingCreatedUTC,
				},
			},
			{
				// Too old — must be filtered out by the since timestamp.
				Kind: "t3",
				Data: redditPost{
					ID:         "old1",
					Name:       "t3_old1",
					Subreddit:  "SideProject",
					Title:      "I wish there was a tool that does X",
					CreatedUTC: oldCreatedUTC,
				},
			},
			{
				// Recent but no frustration phrase — must be filtered out.
				Kind: "t3",
				Data: redditPost{
					ID:         "nomatch1",
					Name:       "t3_nomatch1",
					Subreddit:  "SideProject",
					Title:      "Just sharing my weekend project",
					CreatedUTC: matchingCreatedUTC,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	target, err := url.Parse(srv.URL)
	require.NoError(t, err)

	adapter := &RedditAdapter{
		apiKey:     "test-key",
		apiHost:    "test-host",
		httpClient: &http.Client{Transport: rewriteHostTransport{target: target}},
		logger:     testLogger(),
	}

	since := time.Now().Add(-1 * time.Hour)
	posts, err := adapter.fetchSubreddit(context.Background(), "SideProject", since)
	require.NoError(t, err)
	require.Len(t, posts, 1)

	assert.Equal(t, "reddit:t3_abc123", posts[0].PlatformID)
	assert.Equal(t, "reddit", posts[0].Platform)
	assert.Equal(t, "https://reddit.com/r/SideProject/comments/abc123", posts[0].URL)
	assert.Equal(t, 42, posts[0].Score)
	assert.Equal(t, 7, posts[0].CommentCount)
}

func TestRedditAdapter_FetchSubreddit_FailsOnSuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(rapidAPIPostsResponse{Success: false})
	}))
	defer srv.Close()

	target, err := url.Parse(srv.URL)
	require.NoError(t, err)

	adapter := &RedditAdapter{
		apiKey:     "test-key",
		apiHost:    "test-host",
		httpClient: &http.Client{Transport: rewriteHostTransport{target: target}},
		logger:     testLogger(),
	}

	_, err = adapter.fetchSubreddit(context.Background(), "SideProject", time.Now().Add(-1*time.Hour))
	assert.Error(t, err)
}

// TestRedditAdapter_FetchSince_AggregatesAcrossAllSubreddits proves the full
// FetchSince loop (one matching post per subreddit call) without hitting the
// real RapidAPI quota — every request is served by a local httptest server.
// Shrinks redditRequestDelay so 12 sequential calls don't slow the test suite.
func TestRedditAdapter_FetchSince_AggregatesAcrossAllSubreddits(t *testing.T) {
	originalDelay := redditRequestDelay
	redditRequestDelay = time.Millisecond
	defer func() { redditRequestDelay = originalDelay }()

	matchingCreatedUTC := float64(time.Now().Add(-5 * time.Minute).Unix())
	var requestCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		subreddit := r.URL.Query().Get("subreddit")

		resp := rapidAPIPostsResponse{Success: true}
		resp.Data.Posts = []struct {
			Kind string     `json:"kind"`
			Data redditPost `json:"data"`
		}{
			{
				Kind: "t3",
				Data: redditPost{
					ID:         subreddit + "-post1",
					Name:       "t3_" + subreddit + "post1",
					Subreddit:  subreddit,
					Title:      "I would pay for something that solved this",
					CreatedUTC: matchingCreatedUTC,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	target, err := url.Parse(srv.URL)
	require.NoError(t, err)

	adapter := &RedditAdapter{
		apiKey:     "test-key",
		apiHost:    "test-host",
		httpClient: &http.Client{Transport: rewriteHostTransport{target: target}},
		logger:     testLogger(),
	}

	since := time.Now().Add(-1 * time.Hour)
	posts, err := adapter.FetchSince(context.Background(), since)
	require.NoError(t, err)

	assert.Equal(t, len(targetSubreddits), requestCount, "must call once per target subreddit, never more")
	assert.Len(t, posts, len(targetSubreddits), "one matching post per subreddit must be aggregated")
}

// TestRedditAdapter_FetchSince_ContinuesPastPerSubredditErrors proves a single
// failing subreddit (e.g. a transient 500) doesn't abort the whole fetch cycle.
func TestRedditAdapter_FetchSince_ContinuesPastPerSubredditErrors(t *testing.T) {
	originalDelay := redditRequestDelay
	redditRequestDelay = time.Millisecond
	defer func() { redditRequestDelay = originalDelay }()

	matchingCreatedUTC := float64(time.Now().Add(-5 * time.Minute).Unix())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subreddit := r.URL.Query().Get("subreddit")
		if subreddit == targetSubreddits[0] {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := rapidAPIPostsResponse{Success: true}
		resp.Data.Posts = []struct {
			Kind string     `json:"kind"`
			Data redditPost `json:"data"`
		}{
			{
				Kind: "t3",
				Data: redditPost{
					ID:         subreddit + "-post1",
					Name:       "t3_" + subreddit + "post1",
					Subreddit:  subreddit,
					Title:      "I would pay for something that solved this",
					CreatedUTC: matchingCreatedUTC,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	target, err := url.Parse(srv.URL)
	require.NoError(t, err)

	adapter := &RedditAdapter{
		apiKey:     "test-key",
		apiHost:    "test-host",
		httpClient: &http.Client{Transport: rewriteHostTransport{target: target}},
		logger:     testLogger(),
	}

	posts, err := adapter.FetchSince(context.Background(), time.Now().Add(-1*time.Hour))
	require.NoError(t, err, "one failing subreddit must not fail the whole cycle")
	assert.Len(t, posts, len(targetSubreddits)-1, "all subreddits except the failing one must still be aggregated")
}
