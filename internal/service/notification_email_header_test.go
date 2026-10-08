package service

// Notification subjects now carry user-written text (conversation
// subjects, announcement and topic titles). These tests pin that such
// text can neither inject SMTP headers nor HTML into a digest.

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// startFakeSMTP accepts one SMTP session on localhost and returns the
// raw DATA payload it received.
func startFakeSMTP(t *testing.T) (int, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	got := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		reply := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		reply("220 fake.example ESMTP")
		var data strings.Builder
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					got <- data.String()
					reply("250 queued")
					continue
				}
				data.WriteString(line)
				continue
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "DATA"):
				inData = true
				reply("354 go ahead")
			case strings.HasPrefix(cmd, "QUIT"):
				reply("221 bye")
				return
			default:
				reply("250 ok")
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, got
}

func TestSendEmail_SubjectCannotInjectHeaders(t *testing.T) {
	port, got := startFakeSMTP(t)
	settings := map[string]string{
		"smtp.host": "127.0.0.1", "smtp.port": strconv.Itoa(port),
		"smtp.username": "", "smtp.password": "",
		"smtp.from": "noreply@example.com", "smtp.enabled": "true",
	}
	lookup := func(_ context.Context, key string) (string, error) {
		v, ok := settings[key]
		if !ok {
			return "", fmt.Errorf("unknown key %s", key)
		}
		return v, nil
	}
	svc := NewNotificationDeliveryService(nil, nil, nil, nil, nil, lookup)

	evil := "Question\r\nBcc: victim@example.com\r\n\r\n<a href=\"https://phish.example\">reset password</a>"
	require.NoError(t, svc.SendEmail(context.Background(), "jane@example.com", evil, "<p>body</p>"))

	var raw string
	select {
	case raw = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("fake SMTP server received no message")
	}
	headers, body, found := strings.Cut(raw, "\r\n\r\n")
	require.True(t, found, "message has a header/body separator")
	for _, line := range strings.Split(headers, "\r\n") {
		assert.False(t, strings.HasPrefix(strings.ToLower(line), "bcc:"), "injected header line %q", line)
	}
	assert.Contains(t, headers, "Subject: Question  Bcc: victim@example.com")
	assert.NotContains(t, body, `<a href="https://phish.example">`, "the subject must not inject raw HTML into the body")
}

func TestSanitizeEmailHeaderValue(t *testing.T) {
	assert.Equal(t, "a  b", sanitizeEmailHeaderValue("a\r\nb"))
	assert.Equal(t, "a b", sanitizeEmailHeaderValue("a\nb"))
	assert.Equal(t, "Grade posted: Essay 1", sanitizeEmailHeaderValue("Grade posted: Essay 1"))
	assert.Equal(t, "x", sanitizeEmailHeaderValue("\r\nx\r\n"))
}

func TestBuildDigestBody_EscapesSubjects(t *testing.T) {
	body := buildDigestBody("daily", []models.NotificationDelivery{
		{Subject: `New message from Example User: <img src=x onerror=alert(1)>`, Body: "hello &amp; welcome"},
		{Subject: "Grade posted", Body: "<p>90/100</p>"},
	})
	assert.NotContains(t, body, "<img")
	assert.Contains(t, body, "&lt;img src=x onerror=alert(1)&gt;")
	assert.Contains(t, body, "<p>90/100</p>", "bodies are already sanitized HTML and pass through")
}
