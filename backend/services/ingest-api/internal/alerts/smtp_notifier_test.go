package alerts

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSMTPNotifierSendsEmail(t *testing.T) {
	server := newFakeSMTPServer(t)
	defer server.Close()

	notifier, err := NewSMTPNotifier(SMTPConfig{
		Addr:          server.Addr(),
		From:          "alerts@fractsoul.local",
		To:            []string{"ops@fractsoul.local"},
		SubjectPrefix: "[Test Alert]",
	})
	if err != nil {
		t.Fatalf("new smtp notifier: %v", err)
	}

	_, err = notifier.Notify(context.Background(), PersistedAlert{
		AlertID:          "alert-2",
		RuleID:           "power_spike",
		RuleName:         "Pico de Consumo",
		Severity:         SeverityWarning,
		Status:           StatusOpen,
		Message:          "Consumo fuera de banda",
		SiteID:           "site-cl-01",
		RackID:           "rack-cl-01-01",
		MinerID:          "asic-000002",
		MinerModel:       "S21",
		MetricName:       "power_watts",
		MetricValue:      4300,
		Threshold:        4260,
		Occurrences:      1,
		FirstSeenAt:      time.Now().UTC(),
		LastSeenAt:       time.Now().UTC(),
		SuppressionUntil: time.Now().UTC().Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("notify smtp: %v", err)
	}

	mail := server.WaitForMail(t, 2*time.Second)
	if !strings.Contains(mail, "Subject: [Test Alert] [WARNING] power_spike asic-000002") {
		t.Fatalf("expected subject in email payload, got %s", mail)
	}
	if !strings.Contains(mail, "Alerta detectada") {
		t.Fatalf("expected body marker in email payload")
	}
}

type fakeSMTPServer struct {
	listener net.Listener
	mailCh   chan string
	doneCh   chan struct{}
	wg       sync.WaitGroup
}

func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start fake smtp listener: %v", err)
	}

	server := &fakeSMTPServer{
		listener: ln,
		mailCh:   make(chan string, 4),
		doneCh:   make(chan struct{}),
	}

	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-server.doneCh:
					return
				default:
					return
				}
			}
			server.wg.Add(1)
			go func(c net.Conn) {
				defer server.wg.Done()
				server.handleConn(c)
			}(conn)
		}
	}()

	return server
}

func (s *fakeSMTPServer) Addr() string {
	return s.listener.Addr().String()
}

func (s *fakeSMTPServer) Close() {
	close(s.doneCh)
	_ = s.listener.Close()
	s.wg.Wait()
}

func (s *fakeSMTPServer) WaitForMail(t *testing.T, timeout time.Duration) string {
	t.Helper()

	select {
	case mail := <-s.mailCh:
		return mail
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for fake smtp mail")
		return ""
	}
}

func (s *fakeSMTPServer) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writeLine := func(line string) bool {
		_, err := fmt.Fprintf(conn, "%s\r\n", line)
		return err == nil
	}

	if !writeLine("220 fake-smtp") {
		return
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(command, "EHLO"), strings.HasPrefix(command, "HELO"):
			if !writeLine("250-fake-smtp") {
				return
			}
			if !writeLine("250 OK") {
				return
			}
		case strings.HasPrefix(command, "MAIL FROM"):
			if !writeLine("250 OK") {
				return
			}
		case strings.HasPrefix(command, "RCPT TO"):
			if !writeLine("250 OK") {
				return
			}
		case strings.HasPrefix(command, "DATA"):
			if !writeLine("354 End data with <CR><LF>.<CR><LF>") {
				return
			}
			var payload strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimSpace(dataLine) == "." {
					break
				}
				payload.WriteString(dataLine)
			}
			select {
			case s.mailCh <- payload.String():
			default:
			}
			if !writeLine("250 OK") {
				return
			}
		case strings.HasPrefix(command, "QUIT"):
			_ = writeLine("221 Bye")
			return
		default:
			if !writeLine("250 OK") {
				return
			}
		}
	}
}
