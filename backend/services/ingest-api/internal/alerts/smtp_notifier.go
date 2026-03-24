package alerts

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type SMTPConfig struct {
	Addr          string
	Username      string
	Password      string
	From          string
	To            []string
	SubjectPrefix string
}

type SMTPNotifier struct {
	addr          string
	from          string
	to            []string
	auth          smtp.Auth
	subjectPrefix string
}

func NewSMTPNotifier(cfg SMTPConfig) (*SMTPNotifier, error) {
	addr := strings.TrimSpace(cfg.Addr)
	from := strings.TrimSpace(cfg.From)
	to := compactNonEmpty(cfg.To)

	if addr == "" {
		return nil, fmt.Errorf("smtp addr is required")
	}
	if from == "" {
		return nil, fmt.Errorf("smtp from is required")
	}
	if len(to) == 0 {
		return nil, fmt.Errorf("smtp recipients are required")
	}
	if cfg.SubjectPrefix == "" {
		cfg.SubjectPrefix = "[Fractsoul Alert]"
	}

	var auth smtp.Auth
	username := strings.TrimSpace(cfg.Username)
	password := strings.TrimSpace(cfg.Password)
	if username != "" || password != "" {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("parse smtp addr %s: %w", addr, err)
		}
		auth = smtp.PlainAuth("", username, password, host)
	}

	return &SMTPNotifier{
		addr:          addr,
		from:          from,
		to:            to,
		auth:          auth,
		subjectPrefix: cfg.SubjectPrefix,
	}, nil
}

func (n *SMTPNotifier) Channel() NotificationChannel {
	return ChannelEmail
}

func (n *SMTPNotifier) Notify(_ context.Context, alert PersistedAlert) (DeliveryResult, error) {
	subject := fmt.Sprintf(
		"%s [%s] %s %s",
		n.subjectPrefix,
		strings.ToUpper(string(alert.Severity)),
		alert.RuleID,
		alert.MinerID,
	)
	body := fmt.Sprintf(
		"Alerta detectada\n\n"+
			"Alert ID: %s\n"+
			"Regla: %s (%s)\n"+
			"Severidad: %s\n"+
			"Estado: %s\n"+
			"Mensaje: %s\n"+
			"Sitio/Rack/Miner: %s / %s / %s\n"+
			"Modelo: %s\n"+
			"Metrica: %s = %.3f (umbral %.3f)\n"+
			"Ocurrencias: %d\n"+
			"Primera deteccion: %s\n"+
			"Ultima deteccion: %s\n"+
			"Supresion hasta: %s\n",
		alert.AlertID,
		alert.RuleID,
		alert.RuleName,
		alert.Severity,
		alert.Status,
		alert.Message,
		alert.SiteID,
		alert.RackID,
		alert.MinerID,
		alert.MinerModel,
		alert.MetricName,
		alert.MetricValue,
		alert.Threshold,
		alert.Occurrences,
		alert.FirstSeenAt.Format(time.RFC3339),
		alert.LastSeenAt.Format(time.RFC3339),
		alert.SuppressionUntil.Format(time.RFC3339),
	)

	msg := buildSMTPMessage(n.from, n.to, subject, body)
	if err := smtp.SendMail(n.addr, n.auth, n.from, n.to, []byte(msg)); err != nil {
		return DeliveryResult{
			Destination: strings.Join(n.to, ","),
			Payload: map[string]any{
				"subject": subject,
				"to":      n.to,
			},
		}, fmt.Errorf("send smtp notification: %w", err)
	}

	return DeliveryResult{
		Destination:  strings.Join(n.to, ","),
		ResponseCode: 250,
		Payload: map[string]any{
			"subject": subject,
			"to":      n.to,
		},
	}, nil
}

func buildSMTPMessage(from string, to []string, subject, body string) string {
	headers := []string{
		"From: " + from,
		"To: " + strings.Join(to, ", "),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		`Content-Type: text/plain; charset="UTF-8"`,
	}

	return strings.Join(headers, "\r\n") + "\r\n\r\n" + body
}

func compactNonEmpty(items []string) []string {
	values := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}
