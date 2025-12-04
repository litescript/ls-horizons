package dsn

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// DefaultDSNURL is the official NASA DSN Now XML feed.
	DefaultDSNURL = "https://eyes.nasa.gov/dsn/data/dsn.xml"

	// DefaultTimeout for HTTP requests.
	DefaultTimeout = 30 * time.Second
)

// Fetcher handles HTTP fetching of DSN data.
type Fetcher struct {
	client  *http.Client
	url     string
	timeout time.Duration
}

// FetcherOption configures a Fetcher.
type FetcherOption func(*Fetcher)

// WithURL sets a custom URL for the DSN feed.
func WithURL(url string) FetcherOption {
	return func(f *Fetcher) {
		f.url = url
	}
}

// WithTimeout sets the HTTP request timeout.
func WithTimeout(d time.Duration) FetcherOption {
	return func(f *Fetcher) {
		f.timeout = d
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) FetcherOption {
	return func(f *Fetcher) {
		f.client = client
	}
}

// NewFetcher creates a new DSN data fetcher.
func NewFetcher(opts ...FetcherOption) *Fetcher {
	f := &Fetcher{
		url:     DefaultDSNURL,
		timeout: DefaultTimeout,
	}

	for _, opt := range opts {
		opt(f)
	}

	if f.client == nil {
		f.client = &http.Client{
			Timeout: f.timeout,
		}
	}

	return f
}

// FetchResult contains the result of a fetch operation.
type FetchResult struct {
	Data      *DSNData
	RawBytes  []byte
	FetchedAt time.Time
	Duration  time.Duration
	Error     error
}

// Fetch retrieves and parses the DSN XML feed.
func (f *Fetcher) Fetch(ctx context.Context) FetchResult {
	start := time.Now()
	result := FetchResult{
		FetchedAt: start,
	}

	rawData, err := f.fetchRaw(ctx)
	result.Duration = time.Since(start)
	if err != nil {
		result.Error = err
		return result
	}
	result.RawBytes = rawData

	data, err := Parse(rawData)
	if err != nil {
		result.Error = fmt.Errorf("parse DSN data: %w", err)
		return result
	}
	result.Data = data

	return result
}

// FetchRaw retrieves the raw XML bytes without parsing.
func (f *Fetcher) FetchRaw(ctx context.Context) ([]byte, error) {
	return f.fetchRaw(ctx)
}

func (f *Fetcher) fetchRaw(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "ls-horizons/1.0 (DSN Visualization Tool)")
	req.Header.Set("Accept", "application/xml, text/xml")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch DSN XML: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return body, nil
}

// URL returns the configured feed URL.
func (f *Fetcher) URL() string {
	return f.url
}
