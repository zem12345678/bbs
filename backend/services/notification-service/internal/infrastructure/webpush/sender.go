package webpush

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	webpushlib "github.com/SherClockHolmes/webpush-go"

	domain "notification-service/internal/domain/notification"
)

var ErrUnsafeEndpoint = errors.New("unsafe web push endpoint")

type Sender struct {
	config domain.WebPushConfig
	client *http.Client
}

type payload struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

func NewSender(config domain.WebPushConfig) *Sender {
	return &Sender{config: config, client: newSafeHTTPClient()}
}

func (s *Sender) Send(ctx context.Context, delivery domain.WebPushDelivery) (int, error) {
	if err := validateDispatchEndpoint(ctx, delivery.Endpoint, net.DefaultResolver); err != nil {
		return 0, err
	}
	body, err := marshalPayload(delivery)
	if err != nil {
		return 0, err
	}
	response, err := webpushlib.SendNotificationWithContext(ctx, body, &webpushlib.Subscription{
		Endpoint: delivery.Endpoint,
		Keys: webpushlib.Keys{
			Auth:   delivery.Auth,
			P256dh: delivery.PublicKey,
		},
	}, &webpushlib.Options{
		HTTPClient:      s.client,
		Subscriber:      s.config.Subject,
		VAPIDPublicKey:  s.config.PublicKey,
		VAPIDPrivateKey: s.config.PrivateKey,
		TTL:             60,
	})
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	return response.StatusCode, nil
}

func marshalPayload(delivery domain.WebPushDelivery) ([]byte, error) {
	return json.Marshal(payload{
		ID:    strconv.FormatInt(delivery.Notification.ID, 10),
		Type:  delivery.Notification.Type,
		Title: delivery.Notification.Title,
		Body:  delivery.Notification.Content,
		URL:   "/dashboard/messages",
	})
}

type ipResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

func validateDispatchEndpoint(ctx context.Context, endpoint string, resolver ipResolver) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return ErrUnsafeEndpoint
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return ErrUnsafeEndpoint
	}
	addresses, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	if len(addresses) == 0 {
		return fmt.Errorf("resolve web push endpoint %q: no addresses", host)
	}
	for _, address := range addresses {
		if !isPublicIP(address.IP) {
			return ErrUnsafeEndpoint
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range blockedWebPushPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

var blockedWebPushPrefixes = []netip.Prefix{
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

func newSafeHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&safeDialer{
		resolver: net.DefaultResolver,
		dialer:   &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second},
	}).DialContext
	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("web push redirects are disabled")
		},
	}
}

type safeDialer struct {
	resolver ipResolver
	dialer   *net.Dialer
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
	if len(addresses) == 0 {
		return nil, fmt.Errorf("resolve web push endpoint %q: no addresses", host)
	}
	for _, resolved := range addresses {
		if !isPublicIP(resolved.IP) {
			return nil, ErrUnsafeEndpoint
		}
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
