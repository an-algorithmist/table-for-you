package tools

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"table-for-you/backend/internal/providers"
	"time"
)

func publicIP(ip net.IP) bool {
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}
func publicMenuClient() *http.Client {
	transport := &http.Transport{TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 15 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		if len(ips) == 0 {
			return nil, errors.New("no public address")
		}
		for _, ip := range ips {
			if !publicIP(ip.IP) {
				return nil, errors.New("non-public menu address")
			}
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}}
	return &http.Client{Transport: transport, Timeout: 25 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || !providers.SafeURL(req.URL.String()) {
			return errors.New("unsafe menu redirect")
		}
		return nil
	}}
}

// FetchMenu downloads a bounded public PDF or image. DNS and redirects are checked
// before handing binary source data to a model, so retrieval cannot access private hosts.
func FetchMenu(ctx context.Context, raw string) ([]byte, string, error) {
	if !providers.SafeURL(raw) {
		return nil, "", errors.New("unsafe menu URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "NebulaIQ-Assignment/1.0")
	resp, err := publicMenuClient().Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", errors.New("menu file unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024+1))
	if err != nil || len(data) > 5*1024*1024 {
		return nil, "", errors.New("menu exceeds 5 MiB limit")
	}
	mime := http.DetectContentType(data)
	if strings.HasPrefix(string(data), "%PDF-") {
		mime = "application/pdf"
	}
	if mime != "application/pdf" && mime != "image/png" && mime != "image/jpeg" && mime != "image/webp" {
		return nil, "", errors.New("not a supported menu file")
	}
	return data, mime, nil
}
