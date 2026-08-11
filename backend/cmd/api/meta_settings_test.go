package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookCallbackURLUsesProxyHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "http://backend:8080/api/settings/whatsapp", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "zap.newlifefibra.com.br")
	got := webhookCallbackURL(r)
	if got != "https://zap.newlifefibra.com.br/api/webhooks/whatsapp" {
		t.Fatalf("unexpected callback URL %q", got)
	}
}

func TestRandomVerifyToken(t *testing.T) {
	first, second := randomVerifyToken(), randomVerifyToken()
	if len(first) < 32 || strings.ContainsAny(first, " \t\r\n") {
		t.Fatalf("invalid generated token %q", first)
	}
	if first == second {
		t.Fatal("generated tokens must be unique")
	}
}
