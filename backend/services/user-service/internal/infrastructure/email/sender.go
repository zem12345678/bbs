package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	stdmail "net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	TLSModeNone     = "none"
	TLSModeStartTLS = "starttls"
	TLSModeImplicit = "implicit"

	defaultTimeout = 10 * time.Second
)

var ErrDeliveryDisabled = errors.New("security email delivery is disabled")

type Options struct {
	Enabled         bool          `mapstructure:"enabled"`
	SMTPAddr        string        `mapstructure:"smtpAddr"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	From            string        `mapstructure:"from"`
	FrontendBaseURL string        `mapstructure:"frontendBaseURL"`
	TLSMode         string        `mapstructure:"tlsMode"`
	Timeout         time.Duration `mapstructure:"timeout"`
}

type Sender struct {
	enabled         bool
	smtpAddr        string
	smtpHost        string
	username        string
	password        string
	fromAddress     string
	fromHeader      string
	frontendBaseURL url.URL
	tlsMode         string
	timeout         time.Duration
}

func NewOptions(v *viper.Viper) Options {
	return Options{
		Enabled:         v.GetBool("mail.enabled"),
		SMTPAddr:        v.GetString("mail.smtpAddr"),
		Username:        v.GetString("mail.username"),
		Password:        v.GetString("mail.password"),
		From:            v.GetString("mail.from"),
		FrontendBaseURL: v.GetString("mail.frontendBaseURL"),
		TLSMode:         v.GetString("mail.tlsMode"),
		Timeout:         v.GetDuration("mail.timeout"),
	}
}

func New(options Options) (*Sender, error) {
	options.SMTPAddr = strings.TrimSpace(options.SMTPAddr)
	options.Username = strings.TrimSpace(options.Username)
	options.From = strings.TrimSpace(options.From)
	options.FrontendBaseURL = strings.TrimSpace(options.FrontendBaseURL)
	options.TLSMode = strings.ToLower(strings.TrimSpace(options.TLSMode))
	if options.Timeout <= 0 {
		options.Timeout = defaultTimeout
	}
	if !options.Enabled {
		return &Sender{timeout: options.Timeout}, nil
	}
	if options.SMTPAddr == "" {
		return nil, errors.New("mail.smtpAddr is required when mail delivery is enabled")
	}
	host, _, err := net.SplitHostPort(options.SMTPAddr)
	if err != nil || strings.TrimSpace(host) == "" {
		return nil, errors.New("mail.smtpAddr must include host and port")
	}
	if (options.Username == "") != (strings.TrimSpace(options.Password) == "") {
		return nil, errors.New("mail.username and mail.password must be configured together")
	}
	from, err := stdmail.ParseAddress(options.From)
	if err != nil || from.Address == "" {
		return nil, errors.New("mail.from must be a valid email address")
	}
	frontendBaseURL, err := url.Parse(options.FrontendBaseURL)
	if err != nil || frontendBaseURL.Host == "" || (frontendBaseURL.Scheme != "http" && frontendBaseURL.Scheme != "https") {
		return nil, errors.New("mail.frontendBaseURL must be an absolute HTTP(S) URL")
	}
	if options.TLSMode == "" {
		options.TLSMode = TLSModeStartTLS
	}
	switch options.TLSMode {
	case TLSModeNone, TLSModeStartTLS, TLSModeImplicit:
	default:
		return nil, errors.New("mail.tlsMode must be none, starttls or implicit")
	}
	return &Sender{
		enabled:         true,
		smtpAddr:        options.SMTPAddr,
		smtpHost:        host,
		username:        options.Username,
		password:        options.Password,
		fromAddress:     from.Address,
		fromHeader:      from.String(),
		frontendBaseURL: *frontendBaseURL,
		tlsMode:         options.TLSMode,
		timeout:         options.Timeout,
	}, nil
}

func (s *Sender) Ready() bool {
	return s != nil && s.enabled
}

func (s *Sender) SendPasswordReset(ctx context.Context, recipient, token string, expiresAt time.Time) error {
	resetURL, err := s.actionURL("/user/password/reset", token)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("你正在重置 BBS Community Platform 的密码。\r\n\r\n请在链接有效期内打开以下地址设置新密码：\r\n%s\r\n\r\n链接有效至：%s\r\n如非本人操作，请忽略此邮件。\r\n", resetURL, expiresAt.Local().Format(time.RFC1123))
	return s.send(ctx, recipient, "重置你的 BBS Community Platform 密码", body)
}

func (s *Sender) SendEmailVerification(ctx context.Context, recipient, token string, expiresAt time.Time) error {
	verificationURL, err := s.actionURL("/user/email/verify", token)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("请验证你的 BBS Community Platform 邮箱地址。\r\n\r\n请在链接有效期内打开以下地址完成验证：\r\n%s\r\n\r\n链接有效至：%s\r\n如非本人操作，请忽略此邮件。\r\n", verificationURL, expiresAt.Local().Format(time.RFC1123))
	return s.send(ctx, recipient, "验证你的 BBS Community Platform 邮箱", body)
}

func (s *Sender) actionURL(route, token string) (string, error) {
	if !s.Ready() {
		return "", ErrDeliveryDisabled
	}
	if strings.TrimSpace(token) == "" {
		return "", errors.New("security token is required")
	}
	urlCopy := s.frontendBaseURL
	urlCopy.Path = strings.TrimRight(urlCopy.Path, "/") + "/" + strings.TrimLeft(route, "/")
	urlCopy.RawPath = ""
	query := urlCopy.Query()
	query.Set("token", token)
	urlCopy.RawQuery = query.Encode()
	return urlCopy.String(), nil
}

func (s *Sender) send(ctx context.Context, recipient, subject, body string) error {
	if !s.Ready() {
		return ErrDeliveryDisabled
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	address, err := stdmail.ParseAddress(strings.TrimSpace(recipient))
	if err != nil || address.Address == "" {
		return errors.New("recipient must be a valid email address")
	}
	message := strings.Join([]string{
		"From: " + s.fromHeader,
		"To: " + address.String(),
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
	}, "\r\n")
	return s.deliver(ctx, address.Address, []byte(message))
}

func (s *Sender) deliver(ctx context.Context, recipient string, message []byte) error {
	dialer := net.Dialer{Timeout: s.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", s.smtpAddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(s.timeout)); err != nil {
		return err
	}
	if s.tlsMode == TLSModeImplicit {
		tlsConn := tls.Client(conn, s.tlsConfig())
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return err
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, s.smtpHost)
	if err != nil {
		return err
	}
	defer client.Close()
	if s.tlsMode == TLSModeStartTLS {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return errors.New("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(s.tlsConfig()); err != nil {
			return err
		}
	}
	if s.username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.username, s.password, s.smtpHost)); err != nil {
			return err
		}
	}
	if err := client.Mail(s.fromAddress); err != nil {
		return err
	}
	if err := client.Rcpt(recipient); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (s *Sender) tlsConfig() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.smtpHost}
}
