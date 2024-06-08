package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	fspath "path"
	"time"
)

type Client struct {
	httpClient *http.Client

	url *url.URL

	forward string
}

func NewClient(url *url.URL) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: time.Second * 15,
		},
		url: url,
	}
}

func (c *Client) SetURL(url *url.URL) {
	c.url = url
}

func (c *Client) SetForward(forward string) {
	c.forward = forward
}

func (c *Client) Request(path string) (io.ReadCloser, error) {
	url := new(url.URL)
	*url = *c.url

	if c.forward != "" {
		url.RawQuery = "forward=" + c.forward
	}

	url.Path = fspath.Join(url.Path, path)

	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()

		return nil, fmt.Errorf("request: bad status: %d", resp.StatusCode)
	}

	return resp.Body, nil
}
