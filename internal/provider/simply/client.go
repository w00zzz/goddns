package simply

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultDDNSEndpoint = "https://ddns.simply.com/nic/update"
	defaultAPIEndpoint  = "https://api.simply.com/2"
	defaultHTTPTimeout  = 15 * time.Second
	maxResponseSize     = 4 * 1024
)

type Mode string

const (
	ModeDynDNS2 Mode = "dyndns2"
	ModeREST    Mode = "rest"
)

type Config struct {
	Mode         Mode
	Account      string
	APIKey       string
	Product      string
	RecordID     int
	TTL          int
	HTTPClient   *http.Client
	DDNSEndpoint string
	APIEndpoint  string
}

type Client struct {
	httpClient *http.Client
	account    string
	apiKey     string
	mode       Mode
	product    string
	recordID   int
	ttl        int
	ddnsURL    string
	apiURL     string
}

func NewClient(account, apiKey string, httpClient *http.Client) *Client {
	return newClient(Config{Account: account, APIKey: apiKey, HTTPClient: httpClient})
}

func newClient(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	mode := cfg.Mode
	if mode == "" {
		mode = ModeDynDNS2
	}
	ddnsURL, apiURL := defaultDDNSEndpoint, defaultAPIEndpoint
	if cfg.DDNSEndpoint != "" {
		ddnsURL = cfg.DDNSEndpoint
	}
	if cfg.APIEndpoint != "" {
		apiURL = cfg.APIEndpoint
	}
	return &Client{
		httpClient: cfg.HTTPClient,
		account:    cfg.Account,
		apiKey:     cfg.APIKey,
		mode:       mode,
		product:    cfg.Product,
		recordID:   cfg.RecordID,
		ttl:        cfg.TTL,
		ddnsURL:    ddnsURL,
		apiURL:     apiURL,
	}
}

func (c *Client) update(ctx context.Context, domain, host, recordType, ip string) error {
	if recordType == "" {
		recordType = "A"
	}
	if c.mode == ModeREST {
		return c.updateREST(ctx, domain, host, recordType, ip)
	}
	return c.updateDynDNS2(ctx, domain, host, ip)
}

func (c *Client) updateDynDNS2(ctx context.Context, domain, host, ip string) error {
	hostname := qualifiedHostname(domain, host)
	params := url.Values{
		"hostname": {hostname},
		"domain":   {domain},
		"myip":     {ip},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ddnsURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("create Simply request: %w", err)
	}
	req.SetBasicAuth(c.account, c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Simply request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := readResponse(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Simply returned HTTP status %s", resp.Status)
	}
	code := strings.Fields(string(body))
	if len(code) == 0 {
		return fmt.Errorf("Simply returned an empty response")
	}
	if code[0] != "good" && code[0] != "nochg" {
		return fmt.Errorf("Simply rejected DNS update: %s", code[0])
	}
	return nil
}

type dnsRecord struct {
	ID       int    `json:"record_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Data     string `json:"data"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority"`
	Comment  string `json:"comment"`
}

type dnsRecordsResponse struct {
	Records []dnsRecord `json:"records"`
}

type dnsRecordPayload struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	TTL      *int   `json:"ttl,omitempty"`
	Priority *int   `json:"priority,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

func (c *Client) updateREST(ctx context.Context, domain, host, recordType, ip string) error {
	if c.product == "" {
		return fmt.Errorf("Simply REST product is required")
	}

	recordID := c.recordID
	var record dnsRecord
	if recordID == 0 {
		var err error
		record, err = c.findRecord(ctx, domain, host, recordType)
		if err != nil {
			return err
		}
		recordID = record.ID
	} else {
		record = dnsRecord{Name: qualifiedHostname(domain, host), Type: recordType}
	}
	if recordID == 0 {
		return fmt.Errorf("Simply A record was not found")
	}

	ttl := record.TTL
	if c.ttl > 0 {
		ttl = c.ttl
	}
	payload := dnsRecordPayload{Type: recordType, Name: record.Name, Data: ip}
	if ttl > 0 {
		payload.TTL = &ttl
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Simply DNS record: %w", err)
	}

	endpoint := fmt.Sprintf("%s/my/products/%s/dns/records/%d/", strings.TrimRight(c.apiURL, "/"), url.PathEscape(c.product), recordID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create Simply REST request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Simply REST request failed: %w", err)
	}
	defer resp.Body.Close()
	if _, err := readResponse(resp); err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Simply REST returned HTTP status %s", resp.Status)
	}
	return nil
}

func (c *Client) findRecord(ctx context.Context, domain, host, recordType string) (dnsRecord, error) {
	endpoint := fmt.Sprintf("%s/my/products/%s/dns/records/", strings.TrimRight(c.apiURL, "/"), url.PathEscape(c.product))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return dnsRecord{}, fmt.Errorf("create Simply records request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return dnsRecord{}, fmt.Errorf("Simply records request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := readResponse(resp)
	if err != nil {
		return dnsRecord{}, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return dnsRecord{}, fmt.Errorf("Simply records returned HTTP status %s", resp.Status)
	}
	var result dnsRecordsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return dnsRecord{}, fmt.Errorf("decode Simply records response: %w", err)
	}

	wanted := strings.ToLower(qualifiedHostname(domain, host))
	for _, record := range result.Records {
		if strings.EqualFold(record.Type, recordType) && strings.ToLower(record.Name) == wanted {
			return record, nil
		}
	}
	return dnsRecord{}, fmt.Errorf("Simply %s record %q was not found", recordType, wanted)
}

func qualifiedHostname(domain, host string) string {
	if host == "@" || strings.EqualFold(host, domain) || strings.HasSuffix(strings.ToLower(host), "."+strings.ToLower(domain)) {
		if host == "@" {
			return domain
		}
		return host
	}
	return host + "." + domain
}

func readResponse(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("read Simply response: %w", err)
	}
	if len(body) > maxResponseSize {
		return nil, fmt.Errorf("Simply response exceeds %d bytes", maxResponseSize)
	}
	return body, nil
}
