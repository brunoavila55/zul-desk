package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

func (a *app) ensureMetaSettingsSchema(ctx context.Context) error {
	_, err := a.db.Exec(ctx, `ALTER TABLE app_settings
		ADD COLUMN IF NOT EXISTS whatsapp_verify_token_encrypted TEXT NOT NULL DEFAULT '',
		ADD COLUMN IF NOT EXISTS whatsapp_app_secret_encrypted TEXT NOT NULL DEFAULT ''`)
	return err
}

func randomVerifyToken() string {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "zul-desk-webhook"
	}
	return hex.EncodeToString(raw)
}

func (a *app) metaSecrets(ctx context.Context) (string, string) {
	verifyToken, appSecret := a.cfg.WhatsAppVerifyToken, a.cfg.WhatsAppAppSecret
	var encryptedVerify, encryptedSecret string
	if err := a.db.QueryRow(ctx, `SELECT whatsapp_verify_token_encrypted,whatsapp_app_secret_encrypted FROM app_settings WHERE id=1`).Scan(&encryptedVerify, &encryptedSecret); err != nil {
		return verifyToken, appSecret
	}
	if encryptedVerify != "" {
		if value, err := a.vault.Decrypt(encryptedVerify); err == nil && value != "" {
			verifyToken = value
		}
	}
	if encryptedSecret != "" {
		if value, err := a.vault.Decrypt(encryptedSecret); err == nil {
			appSecret = value
		}
	}
	return verifyToken, appSecret
}

func webhookCallbackURL(r *http.Request) string {
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host + "/api/webhooks/whatsapp"
}

func (a *app) getWhatsAppSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !requireAdmin(w, r) {
		return
	}
	verifyToken, appSecret := a.metaSecrets(r.Context())
	write(w, 200, map[string]any{
		"callback_url":          webhookCallbackURL(r),
		"verify_token":          verifyToken,
		"app_secret_configured": appSecret != "",
	})
}

func (a *app) updateWhatsAppSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !requireAdmin(w, r) {
		return
	}
	var in struct {
		VerifyToken     string `json:"verify_token"`
		AppSecret       string `json:"app_secret"`
		RegenerateToken bool   `json:"regenerate_verify_token"`
		ClearAppSecret  bool   `json:"clear_app_secret"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "dados inválidos")
		return
	}
	verifyToken, _ := a.metaSecrets(r.Context())
	if in.RegenerateToken {
		verifyToken = randomVerifyToken()
	} else if strings.TrimSpace(in.VerifyToken) != "" {
		verifyToken = strings.TrimSpace(in.VerifyToken)
	}
	encryptedVerify, err := a.vault.Encrypt(verifyToken)
	if err != nil {
		fail(w, 500, "não foi possível proteger o token de verificação")
		return
	}
	var encryptedSecret *string
	if in.ClearAppSecret {
		empty := ""
		encryptedSecret = &empty
	} else if strings.TrimSpace(in.AppSecret) != "" {
		value, encryptErr := a.vault.Encrypt(strings.TrimSpace(in.AppSecret))
		if encryptErr != nil {
			fail(w, 500, "não foi possível proteger o App Secret")
			return
		}
		encryptedSecret = &value
	}
	_, err = a.db.Exec(r.Context(), `UPDATE app_settings SET
		whatsapp_verify_token_encrypted=$1,
		whatsapp_app_secret_encrypted=COALESCE($2,whatsapp_app_secret_encrypted),
		updated_by=$3,updated_at=now() WHERE id=1`, encryptedVerify, encryptedSecret, ident(r).ID)
	if err != nil {
		fail(w, 500, "não foi possível salvar a configuração")
		return
	}
	a.audit(r, ident(r).ID, "WHATSAPP_SETTINGS_UPDATED", "app_settings", "1", nil)
	_, appSecret := a.metaSecrets(r.Context())
	write(w, 200, map[string]any{
		"callback_url":          webhookCallbackURL(r),
		"verify_token":          verifyToken,
		"app_secret_configured": appSecret != "",
	})
}
