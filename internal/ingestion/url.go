package ingestion

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const MaxRemoteDocumentBytes = 5 << 20

// FetchURL blocks localhost, private, link-local, multicast and unspecified targets
// both before and during dialing, preventing common SSRF and DNS-rebinding paths.
func FetchURL(ctx context.Context, rawURL string) (content, sourceType, title string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" || u.Hostname() == "" {
		return "", "", "", fmt.Errorf("URL must be absolute HTTP(S)")
	}
	if err = validateHost(ctx, u.Hostname()); err != nil {
		return "", "", "", err
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, splitErr := net.SplitHostPort(address)
		if splitErr != nil {
			return nil, splitErr
		}
		if err := validateHost(ctx, host); err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, address)
	}}
	client := &http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		return validateHost(req.Context(), req.URL.Hostname())
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("User-Agent", "DocMind-ingestion/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("remote server returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxRemoteDocumentBytes+1))
	if err != nil {
		return "", "", "", err
	}
	if len(body) > MaxRemoteDocumentBytes {
		return "", "", "", fmt.Errorf("remote document exceeds 5 MiB")
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	switch {
	case strings.Contains(ct, "text/html"):
		sourceType = "html"
	case strings.Contains(ct, "application/pdf"):
		sourceType = "pdf"
	case strings.Contains(ct, "text/markdown"):
		sourceType = "markdown"
	case strings.Contains(ct, "text/plain"):
		sourceType = "txt"
	default:
		return "", "", "", fmt.Errorf("unsupported remote Content-Type %q", ct)
	}
	title = u.Hostname()
	return string(body), sourceType, title, nil
}
func validateHost(ctx context.Context, host string) error {
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve remote host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("remote host has no addresses")
	}
	for _, ip := range ips {
		if forbiddenIP(ip.IP) {
			return fmt.Errorf("remote URL resolves to a blocked address")
		}
	}
	return nil
}
func forbiddenIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}
