package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CheckLoopbackURL reports whether raw is an http(s) URL whose host is a
// loopback IP. Hostnames are rejected so DNS cannot steer the token off-box.
func CheckLoopbackURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("rvn api URL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("rvn api URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("rvn api URL must be http or https, got %q", u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("rvn api URL must not include userinfo")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("rvn api URL missing host")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("rvn api URL must use a loopback IP, not hostname %q", host)
	}
	if !ip.IsLoopback() {
		return fmt.Errorf("rvn api URL must be loopback, got %s", host)
	}
	return nil
}

func loopbackHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	baseDial := transport.DialContext
	if baseDial == nil {
		d := net.Dialer{Timeout: 10 * time.Second}
		baseDial = d.DialContext
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if err := loopbackDialAddr(addr); err != nil {
			return nil, err
		}
		return baseDial(ctx, network, addr)
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}
			if req.URL == nil {
				return fmt.Errorf("redirect missing URL")
			}
			if err := CheckLoopbackURL(req.URL.String()); err != nil {
				return fmt.Errorf("refusing redirect off loopback: %w", err)
			}
			return nil
		},
	}
}

func loopbackDialAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("refusing non-loopback raven api address %s", addr)
	}
	return nil
}
