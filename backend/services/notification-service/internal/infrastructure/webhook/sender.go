package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	domain "notification-service/internal/domain/notification"
)

var ErrUnsafeEndpoint = errors.New("unsafe webhook endpoint")

type Sender struct {
	config   domain.WebhookConfig
	resolver ipResolver
	client   *http.Client
}

type webhookPayload struct {
	Server    string          `json:"server"`
	HookID    string          `json:"hookId"`
	UserID    string          `json:"userId"`
	EventID   string          `json:"eventId"`
	CreatedAt int64           `json:"createdAt"`
	Type      string          `json:"type"`
	Body      json.RawMessage `json:"body"`
}

func NewSender(config domain.WebhookConfig) *Sender {
	resolver := net.DefaultResolver
	return &Sender{
		config:   config,
		resolver: resolver,
		client:   newSafeHTTPClient(resolver, config.AllowPrivateEndpoints),
	}
}

func (s *Sender) Send(ctx context.Context, delivery domain.WebhookDelivery) (int, error) {
	if err := validateEndpoint(ctx, delivery.URL, s.resolver, s.config.AllowPrivateEndpoints); err != nil {
		return 0, err
	}
	body, err := marshalWebhookPayload(s.config.ServerURL, delivery)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.URL, strings.NewReader(string(body)))
	if err != nil {
		return 0, err
	}
	request.Header.Set("User-Agent", "Misskey-Hooks")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Misskey-Hook-Id", strconv.FormatInt(delivery.WebhookID, 10))
	request.Header.Set("X-Misskey-Hook-Secret", delivery.Secret)
	if serverURL, parseErr := url.Parse(s.config.ServerURL); parseErr == nil {
		request.Header.Set("X-Misskey-Host", serverURL.Host)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
	return response.StatusCode, nil
}

func marshalWebhookPayload(serverURL string, delivery domain.WebhookDelivery) ([]byte, error) {
	if !json.Valid(delivery.Payload) {
		return nil, errors.New("invalid webhook payload")
	}
	return json.Marshal(webhookPayload{
		Server:    strings.TrimRight(strings.TrimSpace(serverURL), "/"),
		HookID:    strconv.FormatInt(delivery.WebhookID, 10),
		UserID:    strconv.FormatInt(delivery.UserID, 10),
		EventID:   delivery.EventID,
		CreatedAt: delivery.CreatedAt.UnixMilli(),
		Type:      delivery.EventType,
		Body:      json.RawMessage(delivery.Payload),
	})
}

type ipResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

func validateEndpoint(ctx context.Context, endpoint string, resolver ipResolver, allowPrivate bool) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return ErrUnsafeEndpoint
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && !(allowPrivate && scheme == "http") {
		return ErrUnsafeEndpoint
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if (host == "localhost" || strings.HasSuffix(host, ".localhost")) && !allowPrivate {
		return ErrUnsafeEndpoint
	}
	addresses, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	return validateResolvedAddresses(host, addresses, allowPrivate)
}

func validateResolvedAddresses(host string, addresses []net.IPAddr, allowPrivate bool) error {
	if len(addresses) == 0 {
		return fmt.Errorf("resolve webhook endpoint %q: no addresses", host)
	}
	for _, address := range addresses {
		if !isAllowedIP(address.IP, allowPrivate) {
			return ErrUnsafeEndpoint
		}
	}
	return nil
}

func isAllowedIP(ip net.IP, allowPrivate bool) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if allowPrivate {
		return address.IsGlobalUnicast() || address.IsLoopback()
	}
	if !address.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.31.196.0/24"),
	netip.MustParsePrefix("192.52.193.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("192.175.48.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("5f00::/16"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func newSafeHTTPClient(resolver ipResolver, allowPrivate bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&safeDialer{
		resolver:     resolver,
		allowPrivate: allowPrivate,
		dialer:       &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second},
	}).DialContext
	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("webhook redirects are disabled")
		},
	}
}

type safeDialer struct {
	resolver     ipResolver
	dialer       *net.Dialer
	allowPrivate bool
}

func (d *safeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrUnsafeEndpoint
	}
	addresses, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if err := validateResolvedAddresses(host, addresses, d.allowPrivate); err != nil {
		return nil, err
	}
	var lastErr error
	for _, resolved := range addresses {
		connection, dialErr := d.dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		lastErr = dialErr
	}
	return nil, lastErr
}
