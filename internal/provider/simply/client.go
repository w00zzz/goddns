package simply

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultEndpoint    = "https://ddns.simply.com/nic/update"
	defaultHTTPTimeout = 15 * time.Second
	maxResponseSize    = 4 * 1024
)

type Client struct {
	httpClient *http.Client
	account    string
	apiKey     string
	endpoint   string
}

func NewClient(account, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Client{
		httpClient: httpClient,
		account:    account,
		apiKey:     apiKey,
		endpoint:   defaultEndpoint,
	}
}

func (c *Client) update(ctx context.Context, domain, host, ip string) error {
	hostname := host
	if host != "@" && !strings.EqualFold(host, domain) && !strings.HasSuffix(strings.ToLower(host), "."+strings.ToLower(domain)) {
		hostname = host + "." + domain
	}

	params := url.Values{
		"hostname": {hostname},
		"domain":   {domain},
		"myip":     {ip},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("create Simply request: %w", err)
	}
	req.SetBasicAuth(c.account, c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Simply request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return fmt.Errorf("read Simply response: %w", err)
	}
	if len(body) > maxResponseSize {
		return fmt.Errorf("Simply response exceeds %d bytes", maxResponseSize)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Simply returned HTTP status %s", resp.Status)
	}

	code := strings.Fields(string(body))
	if len(code) == 0 {
		return fmt.Errorf("Simply returned an empty response")
	}
	switch code[0] {
	case "good", "nochg":
		return nil
	default:
		return fmt.Errorf("Simply rejected DNS update: %s", code[0])
	}
}
