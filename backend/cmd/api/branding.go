package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type branding struct {
	AppName     string  `json:"app_name"`
	CompanyName string  `json:"company_name"`
	LogoURL     *string `json:"logo_url"`
	FaviconURL  *string `json:"favicon_url"`
}

func (a *app) getBranding(w http.ResponseWriter, r *http.Request) {
	var b branding
	err := a.db.QueryRow(r.Context(), `SELECT app_name,company_name,logo_url,favicon_url FROM app_settings WHERE id=1`).Scan(&b.AppName, &b.CompanyName, &b.LogoURL, &b.FaviconURL)
	if err != nil {
		write(w, 200, branding{AppName: "Zul Desk", CompanyName: "New Life"})
		return
	}
	write(w, 200, b)
}

func (a *app) serveUpload(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "filename")
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, "/\\") {
		fail(w, 404, "arquivo não encontrado")
		return
	}
	path := filepath.Join(a.cfg.UploadDir, name)
	if _, err := os.Stat(path); err != nil {
		fail(w, 404, "arquivo não encontrado")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, path)
}

func (a *app) updateBranding(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		fail(w, 400, "formulário inválido ou arquivo maior que 5 MB")
		return
	}
	appName := strings.TrimSpace(r.FormValue("app_name"))
	companyName := strings.TrimSpace(r.FormValue("company_name"))
	if appName == "" || companyName == "" {
		fail(w, 400, "nome do aplicativo e empresa são obrigatórios")
		return
	}
	if len(appName) > 60 || len(companyName) > 100 {
		fail(w, 400, "nome muito longo")
		return
	}
	logoURL, err := a.saveBrandingFile(r, "logo", false)
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	faviconURL, err := a.saveBrandingFile(r, "favicon", true)
	if err != nil {
		fail(w, 422, err.Error())
		return
	}
	_, err = a.db.Exec(r.Context(), `INSERT INTO app_settings(id,app_name,company_name,logo_url,favicon_url,updated_by) VALUES(1,$1,$2,$3,$4,$5) ON CONFLICT(id) DO UPDATE SET app_name=excluded.app_name,company_name=excluded.company_name,logo_url=COALESCE(excluded.logo_url,app_settings.logo_url),favicon_url=COALESCE(excluded.favicon_url,app_settings.favicon_url),updated_by=excluded.updated_by,updated_at=now()`, appName, companyName, logoURL, faviconURL, ident(r).ID)
	if err != nil {
		fail(w, 500, "não foi possível salvar a identidade visual")
		return
	}
	a.audit(r, ident(r).ID, "BRANDING_UPDATED", "app_settings", "", map[string]any{"app_name": appName, "company_name": companyName})
	a.getBranding(w, r)
}

func (a *app) saveBrandingFile(r *http.Request, field string, favicon bool) (*string, error) {
	file, header, err := r.FormFile(field)
	if err == http.ErrMissingFile {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if header.Size > 5<<20 {
		return nil, fmt.Errorf("%s deve ter no máximo 5 MB", field)
	}
	first := make([]byte, 512)
	n, err := io.ReadFull(file, first)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	mime := http.DetectContentType(first[:n])
	extensions := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}
	if favicon {
		extensions["image/x-icon"] = ".ico"
		extensions["image/vnd.microsoft.icon"] = ".ico"
	}
	ext, ok := extensions[mime]
	if !ok {
		return nil, fmt.Errorf("%s deve ser PNG, JPG ou WebP%s", field, func() string {
			if favicon {
				return " ou ICO"
			}
			return ""
		}())
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	name := field + "-" + uuid.NewString() + ext
	target := filepath.Join(a.cfg.UploadDir, name)
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	if _, err = io.Copy(out, io.LimitReader(file, 5<<20)); err != nil {
		return nil, err
	}
	url := "/api/uploads/" + name
	return &url, nil
}
