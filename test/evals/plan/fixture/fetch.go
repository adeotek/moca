// Package fetcher downloads documents over HTTP.
package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client fetches URLs with a timeout.
type Client struct {
	HTTP *http.Client
}

// New returns a client with a 10s timeout.
func New() *Client { return &Client{HTTP: &http.Client{Timeout: 10 * time.Second}} }

// Get returns the body of url; non-2xx statuses are errors.
func (c *Client) Get(url string) ([]byte, error) {
	resp, err := c.HTTP.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}
