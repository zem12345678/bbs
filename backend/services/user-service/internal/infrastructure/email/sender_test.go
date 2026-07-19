package email

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSenderDeliversPasswordResetAndVerificationLinks(t *testing.T) {
	server := newSMTPTestServer(t)
	sender, err := New(Options{
		Enabled:         true,
		SMTPAddr:        server.listener.Addr().String(),
		From:            "BBS Community <noreply@bbs.local>",
		FrontendBaseURL: "https://community.example/app",
		TLSMode:         TLSModeNone,
		Timeout:         time.Second,
	})
	if err != nil {
		t.Fatalf("new sender: %v", err)
	}
	expiresAt := time.Date(2026, time.July, 20, 12, 0, 0, 0, time.UTC)
	if err := sender.SendPasswordReset(context.Background(), "member@example.com", "reset+token", expiresAt); err != nil {
		t.Fatalf("send password reset: %v", err)
	}
	passwordReset := server.nextMessage(t)
	assertMessageLink(t, passwordReset, "https://community.example/app/user/password/reset?token=reset%2Btoken")
	if !strings.Contains(passwordReset, "To: <member@example.com>") {
		t.Fatalf("password reset recipient missing from message: %s", passwordReset)
	}

	if err := sender.SendEmailVerification(context.Background(), "member@example.com", "verify token", expiresAt); err != nil {
		t.Fatalf("send email verification: %v", err)
	}
	verification := server.nextMessage(t)
	assertMessageLink(t, verification, "https://community.example/app/user/email/verify?token=verify+token")
}

func TestSenderValidatesEnabledConfiguration(t *testing.T) {
	tests := []Options{
		{Enabled: true},
		{Enabled: true, SMTPAddr: "localhost", From: "noreply@bbs.local", FrontendBaseURL: "https://community.example"},
		{Enabled: true, SMTPAddr: "127.0.0.1:1025", From: "invalid", FrontendBaseURL: "https://community.example"},
		{Enabled: true, SMTPAddr: "127.0.0.1:1025", From: "noreply@bbs.local", FrontendBaseURL: "community.example"},
		{Enabled: true, SMTPAddr: "127.0.0.1:1025", Username: "user", From: "noreply@bbs.local", FrontendBaseURL: "https://community.example"},
		{Enabled: true, SMTPAddr: "127.0.0.1:1025", From: "noreply@bbs.local", FrontendBaseURL: "https://community.example", TLSMode: "broken"},
	}
	for _, options := range tests {
		if _, err := New(options); err == nil {
			t.Fatalf("New(%+v) error = nil", options)
		}
	}
}

func TestDisabledSenderDoesNotSend(t *testing.T) {
	sender, err := New(Options{})
	if err != nil {
		t.Fatalf("new disabled sender: %v", err)
	}
	if sender.Ready() {
		t.Fatal("disabled sender should not be ready")
	}
	err = sender.SendPasswordReset(context.Background(), "member@example.com", "token", time.Now().Add(time.Hour))
	if !errors.Is(err, ErrDeliveryDisabled) {
		t.Fatalf("disabled sender error = %v, want ErrDeliveryDisabled", err)
	}
}

func assertMessageLink(t *testing.T, message, expected string) {
	t.Helper()
	if !strings.Contains(message, expected) {
		t.Fatalf("message did not contain %q: %s", expected, message)
	}
	links := strings.Fields(message)
	for _, item := range links {
		if !strings.HasPrefix(item, "https://") {
			continue
		}
		parsed, err := url.Parse(item)
		if err != nil {
			t.Fatalf("parse link %q: %v", item, err)
		}
		if parsed.Query().Get("token") == "" {
			t.Fatalf("link token missing: %q", item)
		}
		return
	}
	t.Fatalf("message did not include a URL: %s", message)
}

type smtpTestServer struct {
	listener net.Listener
	messages chan string
	errs     chan error
}

func newSMTPTestServer(t *testing.T) *smtpTestServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen SMTP: %v", err)
	}
	server := &smtpTestServer{
		listener: listener,
		messages: make(chan string, 4),
		errs:     make(chan error, 4),
	}
	go server.acceptLoop()
	t.Cleanup(func() { _ = listener.Close() })
	return server
}

func (s *smtpTestServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go func() {
			if err := s.handle(conn); err != nil {
				s.errs <- err
			}
		}()
	}
}

func (s *smtpTestServer) handle(conn net.Conn) error {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	if _, err := fmt.Fprint(conn, "220 localhost test SMTP\r\n"); err != nil {
		return err
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		command := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(strings.ToUpper(command), "EHLO "), strings.HasPrefix(strings.ToUpper(command), "HELO "):
			if _, err := fmt.Fprint(conn, "250 localhost\r\n"); err != nil {
				return err
			}
		case strings.HasPrefix(strings.ToUpper(command), "MAIL FROM:"), strings.HasPrefix(strings.ToUpper(command), "RCPT TO:"):
			if _, err := fmt.Fprint(conn, "250 accepted\r\n"); err != nil {
				return err
			}
		case strings.EqualFold(command, "DATA"):
			if _, err := fmt.Fprint(conn, "354 send data\r\n"); err != nil {
				return err
			}
			message, err := readSMTPData(reader)
			if err != nil {
				return err
			}
			s.messages <- message
			if _, err := fmt.Fprint(conn, "250 queued\r\n"); err != nil {
				return err
			}
		case strings.EqualFold(command, "QUIT"):
			_, err := fmt.Fprint(conn, "221 bye\r\n")
			return err
		default:
			return fmt.Errorf("unexpected SMTP command %q", command)
		}
	}
}

func readSMTPData(reader *bufio.Reader) (string, error) {
	var message strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		if strings.TrimRight(line, "\r\n") == "." {
			return message.String(), nil
		}
		message.WriteString(line)
	}
}

func (s *smtpTestServer) nextMessage(t *testing.T) string {
	t.Helper()
	select {
	case err := <-s.errs:
		t.Fatalf("SMTP server error: %v", err)
	case message := <-s.messages:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP message")
	}
	return ""
}
