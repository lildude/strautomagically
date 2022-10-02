// Package client implements a generic REST API client.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

var userAgent = "Stratomagically/0.1"

// Client holds configuration items for the REST client and provides methods that interact with the REST API.
type Client struct {
	BaseURL *url.URL

	userAgent string
	client    *http.Client
}

// NewClient returns a new REST API client. If a nil httpClient is
// provided, http.DefaultClient will be used. To use API methods which require
// authentication, provide an http.Client that will perform the authentication
// for you (such as that provided by the golang.org/x/oauth2 library).
func NewClient(baseURL *url.URL, cc *http.Client) *Client {
	if cc == nil {
		cc = http.DefaultClient
	}

	c := &Client{BaseURL: baseURL, userAgent: userAgent, client: cc}
	return c
}

// NewRequest creates an HTTP Request. If a non-nil body is provided
// it will be JSON encoded and included in the request.
func (c *Client) NewRequest(ctx context.Context, method, urlStr string, body interface{}) (*http.Request, error) {
	u, err := c.BaseURL.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		err = enc.Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return req, nil
}

// Do sends a request and returns the response. An error is returned if the request cannot
// be sent or if the API returns an error. If a response is received, the body response body
// is decoded and stored in the value pointed to by v.
func (c *Client) Do(req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	// Anything other than a HTTP 2xx response code is treated as an error.
	if resp.StatusCode >= 300 { //nolint:gomnd
		err = errors.New(http.StatusText(resp.StatusCode))
		return resp, err
	}

	if v != nil && len(data) != 0 {
		err = json.Unmarshal(data, v)

		switch err {
		case nil:
		case io.EOF:
			err = nil
		default:
		}
	}

	return resp, err
}
