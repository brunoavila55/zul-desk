package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/brunoavila55/zul-desk/internal/whatsapp"
	"github.com/go-chi/chi/v5"
)

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if ident(r).Role != "ADMIN" {
		fail(w, http.StatusForbidden, "acesso restrito ao administrador")
		return false
	}
	return true
}

func (a *app) listWhatsAppAccounts(w http.ResponseWriter, r *http.Request) {
	rows, err := queryMaps(r.Context(), a.db, `SELECT id,name,business_account_id,phone_number_id,display_phone_number,api_version,onboarding_type,coexistence,verified_name,quality_rating,platform_status,token_expires_at,last_verified_at,active,created_at,updated_at,(access_token_encrypted<>'') has_token FROM whatsapp_accounts ORDER BY active DESC,name`)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	write(w, 200, map[string]any{"items": rows})
}

func (a *app) createWhatsAppAccount(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var in struct {
		Name               string `json:"name"`
		WABAID             string `json:"business_account_id"`
		PhoneNumberID      string `json:"phone_number_id"`
		DisplayPhoneNumber string `json:"display_phone_number"`
		AccessToken        string `json:"access_token"`
		APIVersion         string `json:"api_version"`
		Coexistence        bool   `json:"coexistence"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.WABAID) == "" || strings.TrimSpace(in.PhoneNumberID) == "" || strings.TrimSpace(in.AccessToken) == "" {
		fail(w, 400, "nome, WABA ID, Phone Number ID e token são obrigatórios")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.WABAID = strings.TrimSpace(in.WABAID)
	in.PhoneNumberID = strings.TrimSpace(in.PhoneNumberID)
	in.DisplayPhoneNumber = strings.TrimSpace(in.DisplayPhoneNumber)
	in.AccessToken = strings.TrimSpace(in.AccessToken)
	if in.APIVersion == "" {
		in.APIVersion = a.cfg.WhatsAppAPIVersion
	}
	encrypted, err := a.vault.Encrypt(in.AccessToken)
	if err != nil {
		fail(w, 500, "não foi possível proteger o token")
		return
	}
	var id string
	err = a.db.QueryRow(r.Context(), `INSERT INTO whatsapp_accounts(name,business_account_id,phone_number_id,display_phone_number,access_token_encrypted,api_version,onboarding_type,coexistence,active,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,true,$9) RETURNING id`, in.Name, in.WABAID, in.PhoneNumberID, in.DisplayPhoneNumber, encrypted, in.APIVersion, func() string {
		if in.Coexistence {
			return "COEXISTENCE"
		}
		return "CLOUD_API"
	}(), in.Coexistence, ident(r).ID).Scan(&id)
	if err != nil {
		fail(w, 409, "este Phone Number ID já está cadastrado")
		return
	}
	a.audit(r, ident(r).ID, "WHATSAPP_ACCOUNT_CREATED", "whatsapp_account", id, map[string]any{"phone_number_id": in.PhoneNumberID, "coexistence": in.Coexistence})
	write(w, 201, map[string]any{"id": id})
}

func (a *app) updateWhatsAppAccount(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var in struct {
		Name               string `json:"name"`
		WABAID             string `json:"business_account_id"`
		PhoneNumberID      string `json:"phone_number_id"`
		DisplayPhoneNumber string `json:"display_phone_number"`
		AccessToken        string `json:"access_token"`
		Active             *bool  `json:"active"`
		Coexistence        *bool  `json:"coexistence"`
		APIVersion         string `json:"api_version"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "dados inválidos")
		return
	}
	var encrypted *string
	if in.AccessToken != "" {
		value, err := a.vault.Encrypt(in.AccessToken)
		if err != nil {
			fail(w, 500, "não foi possível proteger o token")
			return
		}
		encrypted = &value
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE whatsapp_accounts SET
		name=COALESCE(NULLIF($2,''),name),
		business_account_id=COALESCE(NULLIF($3,''),business_account_id),
		phone_number_id=COALESCE(NULLIF($4,''),phone_number_id),
		display_phone_number=COALESCE(NULLIF($5,''),display_phone_number),
		access_token_encrypted=COALESCE($6,access_token_encrypted),
		active=COALESCE($7,active),coexistence=COALESCE($8,coexistence),
		onboarding_type=CASE WHEN COALESCE($8,coexistence) THEN 'COEXISTENCE' ELSE 'CLOUD_API' END,
		api_version=COALESCE(NULLIF($9,''),api_version),updated_at=now() WHERE id=$1`, id, in.Name, in.WABAID, in.PhoneNumberID, in.DisplayPhoneNumber, encrypted, in.Active, in.Coexistence, in.APIVersion)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		fail(w, http.StatusConflict, "este Phone Number ID já está cadastrado")
		return
	}
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "número não encontrado")
		return
	}
	a.audit(r, ident(r).ID, "WHATSAPP_ACCOUNT_UPDATED", "whatsapp_account", id, nil)
	w.WriteHeader(204)
}

func (a *app) accountCredentials(ctx context.Context, id string) (whatsapp.Credentials, error) {
	var encrypted, phoneID, wabaID, version, onboarding string
	err := a.db.QueryRow(ctx, `SELECT access_token_encrypted,phone_number_id,business_account_id,api_version,onboarding_type FROM whatsapp_accounts WHERE id=$1 AND active`, id).Scan(&encrypted, &phoneID, &wabaID, &version, &onboarding)
	if err != nil {
		return whatsapp.Credentials{}, err
	}
	token, err := a.vault.Decrypt(encrypted)
	return whatsapp.Credentials{AccessToken: token, PhoneNumberID: phoneID, WABAID: wabaID, APIVersion: version, Mock: onboarding == "DEMO"}, err
}

func (a *app) testWhatsAppAccount(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	credentials, err := a.accountCredentials(r.Context(), id)
	if err != nil {
		fail(w, 404, "número não encontrado ou token inválido")
		return
	}
	if credentials.Mock {
		write(w, 200, map[string]any{"status": "CONNECTED", "mock": true})
		return
	}
	phone, err := a.wa.GetPhoneNumber(r.Context(), credentials)
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	_, err = a.db.Exec(r.Context(), `UPDATE whatsapp_accounts SET display_phone_number=$2,verified_name=$3,quality_rating=$4,platform_status=$5,last_verified_at=now(),updated_at=now() WHERE id=$1`, id, phone.DisplayPhoneNumber, phone.VerifiedName, phone.QualityRating, phone.Status)
	if err != nil {
		fail(w, 500, "conexão válida, mas não foi possível atualizar o cadastro")
		return
	}
	write(w, 200, map[string]any{"status": "CONNECTED", "phone": phone})
}

func (a *app) syncWhatsAppPhones(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	credentials, err := a.accountCredentials(r.Context(), id)
	if err != nil {
		fail(w, 404, "conta não encontrada")
		return
	}
	phones, err := a.wa.ListPhoneNumbers(r.Context(), credentials)
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	token, err := a.vault.Encrypt(credentials.AccessToken)
	if err != nil {
		fail(w, 500, "erro de criptografia")
		return
	}
	for _, p := range phones {
		_, _ = a.db.Exec(r.Context(), `INSERT INTO whatsapp_accounts(name,business_account_id,phone_number_id,display_phone_number,access_token_encrypted,api_version,onboarding_type,coexistence,verified_name,quality_rating,platform_status,active,created_by,last_verified_at) SELECT COALESCE(NULLIF($1,''),$2),business_account_id,$2,$3,$4,api_version,onboarding_type,coexistence,$5,$6,$7,true,$8,now() FROM whatsapp_accounts WHERE id=$9 ON CONFLICT(phone_number_id) DO UPDATE SET display_phone_number=excluded.display_phone_number,verified_name=excluded.verified_name,quality_rating=excluded.quality_rating,platform_status=excluded.platform_status,last_verified_at=now(),updated_at=now()`, p.VerifiedName, p.ID, p.DisplayPhoneNumber, token, p.VerifiedName, p.QualityRating, p.Status, ident(r).ID, id)
	}
	write(w, 200, map[string]any{"synced": len(phones)})
}

func (a *app) syncWhatsAppTemplates(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	credentials, err := a.accountCredentials(r.Context(), id)
	if err != nil {
		fail(w, 404, "conta não encontrada")
		return
	}
	templates, err := a.wa.ListTemplates(r.Context(), credentials)
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	for _, t := range templates {
		content := ""
		for _, c := range t.Components {
			if c.Type == "BODY" {
				content = c.Text
				break
			}
		}
		raw, _ := json.Marshal(t.Components)
		if content == "" {
			content = string(raw)
		}
		_, _ = a.db.Exec(r.Context(), `INSERT INTO templates(whatsapp_account_id,meta_template_id,name,language,category,status,content) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(whatsapp_account_id,meta_template_id) WHERE meta_template_id IS NOT NULL DO UPDATE SET name=excluded.name,language=excluded.language,category=excluded.category,status=excluded.status,content=excluded.content,updated_at=now()`, id, t.ID, t.Name, t.Language, t.Category, t.Status, content)
	}
	write(w, 200, map[string]any{"synced": len(templates), "at": time.Now()})
}
