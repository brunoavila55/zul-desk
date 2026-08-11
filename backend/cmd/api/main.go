package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brunoavila55/zul-desk/database/migrations"
	"github.com/brunoavila55/zul-desk/internal/config"
	"github.com/brunoavila55/zul-desk/internal/jobs"
	"github.com/brunoavila55/zul-desk/internal/secure"
	"github.com/brunoavila55/zul-desk/internal/whatsapp"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"
	"golang.org/x/crypto/bcrypt"
)

type ctxKey string

const userKey ctxKey = "user"

type identity struct{ ID, Name, Email, Role string }
type app struct {
	cfg           config.Config
	db            *pgxpool.Pool
	queue         *asynq.Client
	log           *slog.Logger
	hub           *hub
	vault         *secure.Vault
	wa            *whatsapp.Client
	loginMu       sync.Mutex
	loginAttempts map[string]loginAttempt
}
type loginAttempt struct {
	Count int
	Reset time.Time
}
type hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]string
}

func newHub() *hub { return &hub{clients: map[*websocket.Conn]string{}} }
func (h *hub) publish(v any) {
	b, _ := json.Marshal(v)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = c.Write(ctx, websocket.MessageText, b)
		cancel()
	}
}

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := config.Validate(cfg); err != nil {
		log.Error("invalid_configuration", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database_connection_failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	for i := 0; i < 20; i++ {
		if db.Ping(ctx) == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err := migrations.Run(ctx, db); err != nil {
		log.Error("database_migration_failed", "error", err)
		os.Exit(1)
	}
	bootstrapPassword(ctx, db, log, cfg)
	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		log.Error("upload_directory_failed", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.MediaDir, 0755); err != nil {
		log.Error("media_directory_failed", "error", err)
		os.Exit(1)
	}
	a := &app{cfg: cfg, db: db, queue: asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr}), log: log, hub: newHub(), vault: secure.NewVault(cfg.CredentialEncryptionKey), wa: whatsapp.New(cfg), loginAttempts: map[string]loginAttempt{}}
	if err := a.ensureMetaSettingsSchema(ctx); err != nil {
		log.Error("whatsapp_settings_schema_failed", "error", err)
		os.Exit(1)
	}
	defer a.queue.Close()
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Timeout(30*time.Second), middleware.Logger)
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"status": "ok", "time": time.Now()})
	})
	r.Get("/api/public/branding", a.getBranding)
	r.Get("/api/uploads/{filename}", a.serveUpload)
	r.Post("/api/auth/login", a.login)
	r.Post("/api/auth/refresh", a.refresh)
	r.Post("/api/auth/logout", a.logout)
	r.Get("/api/webhooks/whatsapp", a.verifyWebhook)
	r.Post("/api/webhooks/whatsapp", a.receiveWebhook)
	r.Group(func(r chi.Router) {
		r.Use(a.auth)
		r.Get("/api/auth/me", a.me)
		r.Get("/ws", a.ws)
		r.Get("/api/customers", a.listCustomers)
		r.Get("/api/customers/{id}", a.getCustomer)
		r.Post("/api/customers", a.createCustomer)
		r.Patch("/api/customers/{id}", a.updateCustomer)
		r.Post("/api/customers/import", a.importCustomers)
		r.Post("/api/customers/{id}/opt-out", a.optOut)
		r.Get("/api/conversations", a.listConversations)
		r.Post("/api/conversations", a.startConversation)
		r.Get("/api/conversations/{id}", a.getConversation)
		r.Post("/api/conversations/{id}/assign", a.assignConversation)
		r.Post("/api/conversations/{id}/close", a.closeConversation)
		r.Get("/api/conversations/{id}/messages", a.listMessages)
		r.Post("/api/conversations/{id}/messages", a.sendMessage)
		r.Post("/api/conversations/{id}/media", a.sendMedia)
		r.Get("/api/messages/{id}/media", a.serveMessageMedia)
		r.Post("/api/conversations/{id}/notes", a.addNote)
		r.Get("/api/templates", a.listTemplates)
		r.Post("/api/templates/sync", a.syncTemplates)
		r.Get("/api/dashboard", a.dashboard)
		r.Get("/api/reports", a.reports)
		r.Get("/api/users", a.listUsers)
		r.Post("/api/users", a.createUser)
		r.Patch("/api/users/{id}", a.updateUser)
		r.Get("/api/groups", a.listGroups)
		r.Post("/api/groups", a.createGroup)
		r.Patch("/api/groups/{id}", a.updateGroup)
		r.Get("/api/whatsapp/accounts", a.listWhatsAppAccounts)
		r.Post("/api/whatsapp/accounts", a.createWhatsAppAccount)
		r.Patch("/api/whatsapp/accounts/{id}", a.updateWhatsAppAccount)
		r.Post("/api/whatsapp/accounts/{id}/test", a.testWhatsAppAccount)
		r.Post("/api/whatsapp/accounts/{id}/sync-phones", a.syncWhatsAppPhones)
		r.Post("/api/whatsapp/accounts/{id}/sync-templates", a.syncWhatsAppTemplates)
		r.Get("/api/settings/whatsapp", a.getWhatsAppSettings)
		r.Patch("/api/settings/whatsapp", a.updateWhatsAppSettings)
		r.Patch("/api/settings/branding", a.updateBranding)
	})
	h := cors.New(cors.Options{AllowedOrigins: []string{cfg.CORSOrigin}, AllowedMethods: []string{"GET", "POST", "PATCH", "OPTIONS"}, AllowedHeaders: []string{"Authorization", "Content-Type"}, AllowCredentials: true}).Handler(r)
	log.Info("api_started", "address", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, h); err != nil {
		log.Error("api_stopped", "error", err)
	}
}

func bootstrapPassword(ctx context.Context, db *pgxpool.Pool, log *slog.Logger, cfg config.Config) {
	password := cfg.BootstrapAdminPassword
	if password == "" && cfg.AppEnv != "production" {
		password = "comercial123"
	}
	if password == "" {
		log.Warn("bootstrap_password_not_configured")
		return
	}
	h, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	tag, err := db.Exec(ctx, "UPDATE users SET password_hash=$1 WHERE password_hash='$bootstrap$'", string(h))
	if err == nil && tag.RowsAffected() > 0 {
		log.Info("development_users_bootstrapped", "count", tag.RowsAffected())
	}
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	write(w, status, map[string]any{"error": msg})
}
func decode(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(v)
}
func ident(r *http.Request) identity { return r.Context().Value(userKey).(identity) }
func scanRows(rows pgx.Rows) ([]map[string]any, error) {
	defer rows.Close()
	fields := rows.FieldDescriptions()
	out := []map[string]any{}
	for rows.Next() {
		vals, e := rows.Values()
		if e != nil {
			return nil, e
		}
		m := map[string]any{}
		for i, v := range vals {
			if fields[i].DataTypeOID == 2950 && v != nil { // PostgreSQL UUID
				switch raw := v.(type) {
				case [16]byte:
					v = uuid.UUID(raw).String()
				case uuid.UUID:
					v = raw.String()
				case []byte:
					if parsed, err := uuid.FromBytes(raw); err == nil {
						v = parsed.String()
					}
				}
			}
			m[string(fields[i].Name)] = v
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type rowQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func queryMaps(ctx context.Context, db rowQuerier, q string, args ...any) ([]map[string]any, error) {
	rows, e := db.Query(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	return scanRows(rows)
}

func (a *app) tokens(u identity) (string, string, error) {
	now := time.Now()
	claims := jwt.MapClaims{"sub": u.ID, "name": u.Name, "email": u.Email, "role": u.Role, "iat": now.Unix(), "exp": now.Add(15 * time.Minute).Unix()}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	access, e := t.SignedString([]byte(a.cfg.JWTSecret))
	if e != nil {
		return "", "", e
	}
	refresh := uuid.NewString() + uuid.NewString()
	sum := sha256.Sum256([]byte(refresh))
	_, e = a.db.Exec(context.Background(), "INSERT INTO refresh_tokens(user_id,token_hash,expires_at) VALUES($1,$2,$3)", u.ID, hex.EncodeToString(sum[:]), now.Add(7*24*time.Hour))
	return access, refresh, e
}
func (a *app) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !a.allowLogin(r.RemoteAddr) {
		fail(w, http.StatusTooManyRequests, "muitas tentativas; aguarde alguns minutos")
		return
	}
	var in struct{ Email, Password string }
	if decode(r, &in) != nil {
		fail(w, 400, "dados inválidos")
		return
	}
	var u identity
	var hash string
	err := a.db.QueryRow(r.Context(), "SELECT id,name,email,role::text,password_hash FROM users WHERE lower(email)=lower($1) AND active", in.Email).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		fail(w, 401, "e-mail ou senha inválidos")
		return
	}
	a.clearLoginAttempts(r.RemoteAddr)
	access, refresh, err := a.tokens(u)
	if err != nil {
		fail(w, 500, "não foi possível iniciar a sessão")
		return
	}
	a.audit(r, u.ID, "LOGIN", "user", u.ID, nil)
	write(w, 200, map[string]any{"access_token": access, "refresh_token": refresh, "expires_in": 900, "user": u})
}
func (a *app) allowLogin(address string) bool {
	now := time.Now()
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	attempt := a.loginAttempts[address]
	if attempt.Reset.Before(now) {
		attempt = loginAttempt{Reset: now.Add(15 * time.Minute)}
	}
	attempt.Count++
	a.loginAttempts[address] = attempt
	return attempt.Count <= 10
}
func (a *app) clearLoginAttempts(address string) {
	a.loginMu.Lock()
	delete(a.loginAttempts, address)
	a.loginMu.Unlock()
}
func (a *app) refresh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "token ausente")
		return
	}
	sum := sha256.Sum256([]byte(in.RefreshToken))
	var u identity
	var tokenID string
	err := a.db.QueryRow(r.Context(), `SELECT rt.id,u.id,u.name,u.email,u.role::text FROM refresh_tokens rt JOIN users u ON u.id=rt.user_id WHERE rt.token_hash=$1 AND rt.revoked_at IS NULL AND rt.expires_at>now() AND u.active`, hex.EncodeToString(sum[:])).Scan(&tokenID, &u.ID, &u.Name, &u.Email, &u.Role)
	if err != nil {
		fail(w, 401, "sessão expirada")
		return
	}
	_, _ = a.db.Exec(r.Context(), "UPDATE refresh_tokens SET revoked_at=now() WHERE id=$1", tokenID)
	access, refresh, e := a.tokens(u)
	if e != nil {
		fail(w, 500, "erro ao renovar sessão")
		return
	}
	write(w, 200, map[string]any{"access_token": access, "refresh_token": refresh, "expires_in": 900, "user": u})
}
func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = decode(r, &in)
	sum := sha256.Sum256([]byte(in.RefreshToken))
	_, _ = a.db.Exec(r.Context(), "UPDATE refresh_tokens SET revoked_at=now() WHERE token_hash=$1", hex.EncodeToString(sum[:]))
	w.WriteHeader(204)
}
func (a *app) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if raw == "" && r.URL.Path == "/ws" {
			raw = r.URL.Query().Get("token")
		}
		t, e := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("algoritmo inválido")
			}
			return []byte(a.cfg.JWTSecret), nil
		})
		if e != nil || !t.Valid {
			fail(w, 401, "autenticação necessária")
			return
		}
		c := t.Claims.(jwt.MapClaims)
		u := identity{ID: fmt.Sprint(c["sub"]), Name: fmt.Sprint(c["name"]), Email: fmt.Sprint(c["email"]), Role: fmt.Sprint(c["role"])}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}
func (a *app) me(w http.ResponseWriter, r *http.Request) { write(w, 200, ident(r)) }

func (a *app) listCustomers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 50
	if x, _ := strconv.Atoi(r.URL.Query().Get("limit")); x > 0 && x <= 100 {
		limit = x
	}
	rows, e := queryMaps(r.Context(), a.db, `SELECT c.id,c.external_id,c.name,c.phone,c.customer_since,c.product,c.city,c.tags,c.whatsapp_opt_in,c.opt_in_date,c.opt_in_source,c.assigned_user_id,u.name assigned_user_name,EXISTS(SELECT 1 FROM opt_outs o WHERE o.customer_id=c.id) opted_out,EXISTS(SELECT 1 FROM conversations cv WHERE cv.customer_id=c.id AND cv.status<>'CLOSED') has_active_conversation FROM customers c LEFT JOIN users u ON u.id=c.assigned_user_id WHERE c.active AND ($1='' OR c.name ILIKE '%'||$1||'%' OR c.phone ILIKE '%'||$1||'%' OR c.external_id ILIKE '%'||$1||'%' OR c.document ILIKE '%'||$1||'%') ORDER BY c.name LIMIT $2`, q, limit)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	write(w, 200, map[string]any{"items": rows})
}
func (a *app) getCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rows, e := queryMaps(r.Context(), a.db, `SELECT c.*,u.name assigned_user_name,EXISTS(SELECT 1 FROM opt_outs o WHERE o.customer_id=c.id) opted_out FROM customers c LEFT JOIN users u ON u.id=c.assigned_user_id WHERE c.id=$1`, id)
	if e != nil || len(rows) == 0 {
		fail(w, 404, "cliente não encontrado")
		return
	}
	history, _ := queryMaps(r.Context(), a.db, `SELECT id,status::text,result,started_at,closed_at,template_name FROM conversations WHERE customer_id=$1 ORDER BY started_at DESC`, id)
	notes, _ := queryMaps(r.Context(), a.db, `SELECT n.id,n.content,n.created_at,u.name user_name FROM notes n JOIN users u ON u.id=n.user_id WHERE n.customer_id=$1 ORDER BY n.created_at DESC`, id)
	write(w, 200, map[string]any{"customer": rows[0], "history": history, "notes": notes})
}
func (a *app) createCustomer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ExternalID    string `json:"external_id"`
		Name          string `json:"name"`
		Phone         string `json:"phone"`
		Document      string `json:"document"`
		CustomerSince string `json:"customer_since"`
		Product       string `json:"product"`
		City          string `json:"city"`
		OptInSource   string `json:"opt_in_source"`
		WhatsAppOptIn bool   `json:"whatsapp_opt_in"`
	}
	if decode(r, &in) != nil || in.Name == "" || in.Phone == "" {
		fail(w, 400, "nome e telefone são obrigatórios")
		return
	}
	phone := digits(in.Phone)
	var id string
	e := a.db.QueryRow(r.Context(), `INSERT INTO customers(external_id,name,phone,document,customer_since,product,city,whatsapp_opt_in,opt_in_date,opt_in_source) VALUES(NULLIF($1,''),$2,$3,NULLIF($4,''),NULLIF($5,'')::date,NULLIF($6,''),NULLIF($7,''),$8,CASE WHEN $8 THEN now() END,NULLIF($9,'')) RETURNING id`, in.ExternalID, in.Name, phone, in.Document, in.CustomerSince, in.Product, in.City, in.WhatsAppOptIn, in.OptInSource).Scan(&id)
	if e != nil {
		fail(w, 409, "cliente ou telefone já cadastrado")
		return
	}
	a.audit(r, ident(r).ID, "CUSTOMER_CREATED", "customer", id, nil)
	write(w, 201, map[string]any{"id": id})
}
func (a *app) updateCustomer(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	if decode(r, &in) != nil {
		fail(w, 400, "dados inválidos")
		return
	}
	allowed := map[string]bool{"name": true, "phone": true, "document": true, "product": true, "city": true, "whatsapp_opt_in": true, "opt_in_source": true, "assigned_user_id": true}
	sets := []string{}
	args := []any{}
	for k, v := range in {
		if allowed[k] {
			args = append(args, v)
			sets = append(sets, fmt.Sprintf("%s=$%d", k, len(args)))
		}
	}
	if len(sets) == 0 {
		fail(w, 400, "nenhum campo atualizável")
		return
	}
	args = append(args, chi.URLParam(r, "id"))
	q := "UPDATE customers SET " + strings.Join(sets, ",") + ",updated_at=now() WHERE id=$" + strconv.Itoa(len(args))
	tag, e := a.db.Exec(r.Context(), q, args...)
	if e != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "cliente não encontrado")
		return
	}
	a.audit(r, ident(r).ID, "CUSTOMER_UPDATED", "customer", chi.URLParam(r, "id"), in)
	w.WriteHeader(204)
}
func (a *app) importCustomers(w http.ResponseWriter, r *http.Request) {
	f, _, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, "arquivo CSV obrigatório")
		return
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.FieldsPerRecord = -1
	records, e := rd.ReadAll()
	if e != nil || len(records) < 2 {
		fail(w, 400, "CSV inválido")
		return
	}
	headers := map[string]int{}
	for i, h := range records[0] {
		headers[strings.ToLower(strings.TrimSpace(h))] = i
	}
	count := 0
	for _, row := range records[1:] {
		get := func(k string) string {
			i, ok := headers[k]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		opt, _ := strconv.ParseBool(get("opt_in"))
		_, e = a.db.Exec(r.Context(), `INSERT INTO customers(external_id,name,phone,customer_since,product,city,whatsapp_opt_in,opt_in_date,opt_in_source) VALUES(NULLIF($1,''),$2,$3,NULLIF($4,'')::date,NULLIF($5,''),NULLIF($6,''),$7,CASE WHEN $7 THEN now() END,'Importação CSV') ON CONFLICT(phone) DO UPDATE SET name=excluded.name,product=excluded.product,city=excluded.city,updated_at=now()`, get("codigo"), get("nome"), digits(get("telefone")), get("cliente_desde"), get("plano"), get("cidade"), opt)
		if e == nil {
			count++
		}
	}
	a.audit(r, ident(r).ID, "CUSTOMERS_IMPORTED", "customer", "", map[string]any{"count": count})
	write(w, 200, map[string]any{"imported": count, "total": len(records) - 1})
}
func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var templateParameterPattern = regexp.MustCompile(`\{\{(\d+)\}\}`)

func templateParameterCount(content string) int {
	max := 0
	for _, match := range templateParameterPattern.FindAllStringSubmatch(content, -1) {
		value, _ := strconv.Atoi(match[1])
		if value > max {
			max = value
		}
	}
	return max
}

func (a *app) optOut(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	var in struct{ Reason string }
	_ = decode(r, &in)
	id := chi.URLParam(r, "id")
	_, e := a.db.Exec(r.Context(), "INSERT INTO opt_outs(customer_id,reason,created_by) VALUES($1,$2,$3) ON CONFLICT(customer_id) DO UPDATE SET reason=excluded.reason,created_by=excluded.created_by,created_at=now()", id, in.Reason, u.ID)
	if e != nil {
		fail(w, 400, "não foi possível registrar opt-out")
		return
	}
	a.audit(r, u.ID, "OPT_OUT_ADDED", "customer", id, nil)
	w.WriteHeader(204)
}

func (a *app) listConversations(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	scope := r.URL.Query().Get("scope")
	if scope != "mine" && u.Role == "AGENT" {
		scope = "mine"
	}
	status := r.URL.Query().Get("status")
	rows, e := queryMaps(r.Context(), a.db, `SELECT c.id,c.status::text,c.started_at,c.updated_at,c.service_window_expires_at,c.assigned_user_id,c.result,c.whatsapp_account_id,wa.name whatsapp_account_name,wa.display_phone_number whatsapp_display_phone,cu.id customer_id,cu.name customer_name,cu.phone,cu.product,cu.city,cu.customer_since,u.name agent_name,(SELECT body FROM messages WHERE conversation_id=c.id ORDER BY created_at DESC LIMIT 1) last_message,(SELECT created_at FROM messages WHERE conversation_id=c.id ORDER BY created_at DESC LIMIT 1) last_message_at,(SELECT count(*) FROM messages WHERE conversation_id=c.id AND sender_type='CUSTOMER' AND status<>'READ') unread FROM conversations c JOIN customers cu ON cu.id=c.customer_id JOIN users u ON u.id=c.assigned_user_id LEFT JOIN whatsapp_accounts wa ON wa.id=c.whatsapp_account_id WHERE ($1<>'mine' OR c.assigned_user_id=$2) AND ($3='' OR c.status::text=$3) ORDER BY COALESCE((SELECT created_at FROM messages WHERE conversation_id=c.id ORDER BY created_at DESC LIMIT 1),c.updated_at) DESC`, scope, u.ID, status)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	write(w, 200, map[string]any{"items": rows})
}
func (a *app) startConversation(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	var in struct {
		CustomerID        string `json:"customer_id"`
		WhatsAppAccountID string `json:"whatsapp_account_id"`
		TemplateName      string `json:"template_name"`
		Body              string `json:"body"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "dados inválidos")
		return
	}
	if in.WhatsAppAccountID == "" {
		fail(w, 400, "selecione o número do WhatsApp")
		return
	}
	tx, e := a.db.Begin(r.Context())
	if e != nil {
		fail(w, 500, "erro interno")
		return
	}
	defer tx.Rollback(r.Context())
	var phone, name string
	var customerSince *time.Time
	var optIn, optedOut bool
	e = tx.QueryRow(r.Context(), `SELECT c.phone,c.name,c.customer_since,c.whatsapp_opt_in,EXISTS(SELECT 1 FROM opt_outs WHERE customer_id=c.id) FROM customers c WHERE c.id=$1 AND c.active FOR UPDATE`, in.CustomerID).Scan(&phone, &name, &customerSince, &optIn, &optedOut)
	if e != nil {
		fail(w, 404, "cliente não encontrado")
		return
	}
	if !optIn || optedOut {
		fail(w, 422, "cliente sem autorização para contato comercial")
		return
	}
	var existing string
	e = tx.QueryRow(r.Context(), "SELECT id FROM conversations WHERE customer_id=$1 AND status<>'CLOSED'", in.CustomerID).Scan(&existing)
	if e == nil {
		fail(w, 409, "este cliente já possui atendimento ativo")
		return
	}
	if in.TemplateName == "" {
		fail(w, 422, "uma conversa fora da janela deve começar com template aprovado")
		return
	}
	var approved bool
	var templateLanguage, templateContent string
	e = tx.QueryRow(r.Context(), "SELECT status='APPROVED',language,content FROM templates WHERE name=$1 AND (whatsapp_account_id=$2 OR whatsapp_account_id IS NULL)", in.TemplateName, in.WhatsAppAccountID).Scan(&approved, &templateLanguage, &templateContent)
	if e != nil || !approved {
		fail(w, 422, "template não aprovado")
		return
	}
	parameterCount := templateParameterCount(templateContent)
	if parameterCount > 3 {
		fail(w, 422, "este template exige mais variáveis do que o fluxo atual suporta")
		return
	}
	var cid, mid string
	var accountActive bool
	if err := tx.QueryRow(r.Context(), `SELECT active FROM whatsapp_accounts WHERE id=$1`, in.WhatsAppAccountID).Scan(&accountActive); err != nil || !accountActive {
		fail(w, 422, "número do WhatsApp indisponível")
		return
	}
	e = tx.QueryRow(r.Context(), `INSERT INTO conversations(customer_id,assigned_user_id,whatsapp_account_id,status,template_name) VALUES($1,$2,$3,'WAITING_CUSTOMER',$4) RETURNING id`, in.CustomerID, u.ID, in.WhatsAppAccountID, in.TemplateName).Scan(&cid)
	if e != nil {
		fail(w, 409, "este cliente já possui atendimento ativo")
		return
	}
	if strings.TrimSpace(in.Body) == "" {
		fail(w, 400, "mensagem não pode estar vazia")
		return
	}
	e = tx.QueryRow(r.Context(), `INSERT INTO messages(conversation_id,sender_type,user_id,type,body,status) VALUES($1,'AGENT',$2,'TEMPLATE',$3,'PENDING') RETURNING id`, cid, u.ID, in.Body).Scan(&mid)
	if e != nil {
		fail(w, 500, "não foi possível registrar a mensagem")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 500, "não foi possível iniciar o atendimento")
		return
	}
	firstName := strings.Fields(name)[0]
	tenure := "cliente"
	if customerSince != nil {
		tenure = fmt.Sprintf("%d anos", time.Now().Year()-customerSince.Year())
	}
	templateParams := []string{firstName, u.Name, tenure}[:parameterCount]
	task, _ := jobs.NewSendMessage(jobs.SendMessagePayload{MessageID: mid, ConversationID: cid, WhatsAppAccountID: in.WhatsAppAccountID, Phone: phone, Body: in.Body, Template: in.TemplateName, TemplateLanguage: templateLanguage, TemplateParams: templateParams})
	_, e = a.queue.Enqueue(task, asynq.Queue("critical"))
	if e != nil {
		_, _ = a.db.Exec(r.Context(), "UPDATE messages SET status='FAILED',error_message=$2 WHERE id=$1", mid, "fila indisponível")
	}
	a.audit(r, u.ID, "CONVERSATION_STARTED", "conversation", cid, map[string]any{"customer_id": in.CustomerID, "template": in.TemplateName})
	a.hub.publish(map[string]any{"event": "conversation.created", "conversation_id": cid})
	write(w, 201, map[string]any{"id": cid, "message_id": mid})
}
func (a *app) getConversation(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	id := chi.URLParam(r, "id")
	rows, e := queryMaps(r.Context(), a.db, `SELECT c.*,c.status::text status,cu.name customer_name,cu.phone,cu.external_id,cu.product,cu.city,cu.customer_since,cu.tags,cu.whatsapp_opt_in,u.name agent_name,wa.name whatsapp_account_name,wa.display_phone_number whatsapp_display_phone FROM conversations c JOIN customers cu ON cu.id=c.customer_id JOIN users u ON u.id=c.assigned_user_id LEFT JOIN whatsapp_accounts wa ON wa.id=c.whatsapp_account_id WHERE c.id=$1 AND (c.assigned_user_id=$2 OR $3<>'AGENT')`, id, u.ID, u.Role)
	if e != nil || len(rows) == 0 {
		fail(w, 404, "atendimento não encontrado")
		return
	}
	msgs, _ := queryMaps(r.Context(), a.db, `SELECT m.id,m.sender_type::text,m.user_id,m.type,m.body,m.status::text,m.sent_at,m.delivered_at,m.read_at,m.error_message,m.created_at,m.media_mime_type,m.media_filename,m.media_size,(m.media_path IS NOT NULL) media_ready,u.name user_name FROM messages m LEFT JOIN users u ON u.id=m.user_id WHERE m.conversation_id=$1 ORDER BY m.created_at`, id)
	notes, _ := queryMaps(r.Context(), a.db, `SELECT n.id,n.content,n.created_at,u.name user_name FROM notes n JOIN users u ON u.id=n.user_id WHERE n.conversation_id=$1 ORDER BY n.created_at DESC`, id)
	write(w, 200, map[string]any{"conversation": rows[0], "messages": msgs, "notes": notes})
}
func (a *app) listMessages(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	rows, e := queryMaps(r.Context(), a.db, `SELECT m.id,m.sender_type::text,m.user_id,m.type,m.body,m.status::text,m.sent_at,m.delivered_at,m.read_at,m.error_message,m.created_at,m.media_mime_type,m.media_filename,m.media_size,(m.media_path IS NOT NULL) media_ready,u.name user_name FROM messages m JOIN conversations c ON c.id=m.conversation_id LEFT JOIN users u ON u.id=m.user_id WHERE m.conversation_id=$1 AND (c.assigned_user_id=$2 OR $3<>'AGENT') ORDER BY m.created_at`, chi.URLParam(r, "id"), u.ID, u.Role)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	write(w, 200, map[string]any{"items": rows})
}
func (a *app) sendMessage(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	cid := chi.URLParam(r, "id")
	var in struct{ Body string }
	if decode(r, &in) != nil || strings.TrimSpace(in.Body) == "" {
		fail(w, 400, "mensagem não pode estar vazia")
		return
	}
	var phone, status, whatsappAccountID string
	var expires *time.Time
	e := a.db.QueryRow(r.Context(), `SELECT cu.phone,c.status::text,c.whatsapp_account_id,c.service_window_expires_at FROM conversations c JOIN customers cu ON cu.id=c.customer_id WHERE c.id=$1 AND c.assigned_user_id=$2`, cid, u.ID).Scan(&phone, &status, &whatsappAccountID, &expires)
	if e != nil {
		fail(w, 403, "atendimento não encontrado ou atribuído a outro vendedor")
		return
	}
	if status == "CLOSED" {
		fail(w, 422, "atendimento encerrado")
		return
	}
	if expires == nil || expires.Before(time.Now()) {
		fail(w, 422, "janela de atendimento encerrada; use um template aprovado")
		return
	}
	var mid string
	e = a.db.QueryRow(r.Context(), `INSERT INTO messages(conversation_id,sender_type,user_id,body,status) VALUES($1,'AGENT',$2,$3,'PENDING') RETURNING id`, cid, u.ID, in.Body).Scan(&mid)
	if e != nil {
		fail(w, 500, "não foi possível registrar a mensagem")
		return
	}
	_, _ = a.db.Exec(r.Context(), "UPDATE conversations SET status='WAITING_CUSTOMER',updated_at=now() WHERE id=$1", cid)
	task, _ := jobs.NewSendMessage(jobs.SendMessagePayload{MessageID: mid, ConversationID: cid, WhatsAppAccountID: whatsappAccountID, Phone: phone, Body: in.Body})
	_, e = a.queue.Enqueue(task, asynq.Queue("critical"))
	if e != nil {
		_, _ = a.db.Exec(r.Context(), "UPDATE messages SET status='FAILED',error_message='fila indisponível' WHERE id=$1", mid)
	}
	a.audit(r, u.ID, "MESSAGE_SENT", "message", mid, map[string]any{"conversation_id": cid})
	a.hub.publish(map[string]any{"event": "message.created", "conversation_id": cid, "message_id": mid})
	write(w, 202, map[string]any{"id": mid, "status": "PENDING"})
}
func (a *app) assignConversation(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	var in struct {
		UserID string `json:"user_id"`
	}
	if decode(r, &in) != nil {
		fail(w, 400, "responsável inválido")
		return
	}
	id := chi.URLParam(r, "id")
	var targetActive bool
	if e := a.db.QueryRow(r.Context(), "SELECT active FROM users WHERE id=$1", in.UserID).Scan(&targetActive); e != nil || !targetActive {
		fail(w, 400, "novo responsável inválido ou inativo")
		return
	}
	tag, e := a.db.Exec(r.Context(), "UPDATE conversations SET assigned_user_id=$1,updated_at=now() WHERE id=$2 AND status<>'CLOSED' AND (assigned_user_id=$3 OR $4<>'AGENT')", in.UserID, id, u.ID, u.Role)
	if e != nil || tag.RowsAffected() == 0 {
		fail(w, 403, "você só pode transferir conversas sob sua responsabilidade")
		return
	}
	a.audit(r, u.ID, "CONVERSATION_TRANSFERRED", "conversation", id, map[string]any{"to": in.UserID})
	a.hub.publish(map[string]any{"event": "conversation.assigned", "conversation_id": id, "user_id": in.UserID})
	w.WriteHeader(204)
}
func (a *app) closeConversation(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	var in struct{ Result, Note string }
	if decode(r, &in) != nil || in.Result == "" {
		fail(w, 400, "resultado obrigatório")
		return
	}
	id := chi.URLParam(r, "id")
	tag, e := a.db.Exec(r.Context(), "UPDATE conversations SET status='CLOSED',result=$1,result_note=$2,closed_at=now(),updated_at=now() WHERE id=$3 AND status<>'CLOSED' AND (assigned_user_id=$4 OR $5<>'AGENT')", in.Result, in.Note, id, u.ID, u.Role)
	if e != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "atendimento não encontrado")
		return
	}
	a.audit(r, u.ID, "CONVERSATION_CLOSED", "conversation", id, map[string]any{"result": in.Result})
	a.hub.publish(map[string]any{"event": "conversation.closed", "conversation_id": id})
	w.WriteHeader(204)
}
func (a *app) addNote(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	var in struct{ Content string }
	if decode(r, &in) != nil || strings.TrimSpace(in.Content) == "" {
		fail(w, 400, "nota não pode estar vazia")
		return
	}
	cid := chi.URLParam(r, "id")
	var customerID, id string
	e := a.db.QueryRow(r.Context(), `INSERT INTO notes(customer_id,conversation_id,user_id,content) SELECT customer_id,id,$2,$3 FROM conversations WHERE id=$1 AND (assigned_user_id=$2 OR $4<>'AGENT') RETURNING id,customer_id`, cid, u.ID, in.Content, u.Role).Scan(&id, &customerID)
	if e != nil {
		fail(w, 404, "atendimento não encontrado")
		return
	}
	write(w, 201, map[string]any{"id": id})
}

func (a *app) listTemplates(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("whatsapp_account_id")
	rows, e := queryMaps(r.Context(), a.db, "SELECT id,whatsapp_account_id,name,language,category,status,content,updated_at FROM templates WHERE ($1='' OR whatsapp_account_id=$1::uuid OR whatsapp_account_id IS NULL) ORDER BY name", accountID)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	write(w, 200, map[string]any{"items": rows})
}
func (a *app) syncTemplates(w http.ResponseWriter, r *http.Request) {
	if ident(r).Role != "ADMIN" {
		fail(w, 403, "acesso restrito")
		return
	}
	write(w, 202, map[string]any{"status": "queued", "message": "sincronização será executada pelo worker"})
}
func (a *app) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, e := queryMaps(r.Context(), a.db, `SELECT u.id,u.name,u.email,u.role::text,u.active,COALESCE(jsonb_agg(m.group_id::text) FILTER(WHERE m.group_id IS NOT NULL),'[]'::jsonb) group_ids FROM users u LEFT JOIN user_group_members m ON m.user_id=u.id GROUP BY u.id ORDER BY u.name`)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	write(w, 200, map[string]any{"items": rows})
}
func (a *app) dashboard(w http.ResponseWriter, r *http.Request) {
	rows, e := queryMaps(r.Context(), a.db, `SELECT count(*) FILTER(WHERE started_at::date=current_date) conversations_started,count(*) FILTER(WHERE status<>'CLOSED') open_conversations,count(*) FILTER(WHERE closed_at::date=current_date) closed_conversations,count(*) FILTER(WHERE closed_at::date=current_date AND result='SALE') sales FROM conversations`)
	if e != nil {
		fail(w, 500, e.Error())
		return
	}
	agents, _ := queryMaps(r.Context(), a.db, `SELECT u.id,u.name,count(c.id) FILTER(WHERE c.started_at::date=current_date) contacts,count(c.id) FILTER(WHERE c.closed_at::date=current_date AND c.result='SALE') sales FROM users u LEFT JOIN conversations c ON c.assigned_user_id=u.id WHERE u.role='AGENT' GROUP BY u.id,u.name ORDER BY contacts DESC`)
	responses := 0
	_ = a.db.QueryRow(r.Context(), `SELECT count(DISTINCT conversation_id) FROM messages WHERE sender_type='CUSTOMER' AND created_at::date=current_date`).Scan(&responses)
	d := rows[0]
	d["responses"] = responses
	write(w, 200, map[string]any{"summary": d, "agents": agents})
}

func (a *app) ws(w http.ResponseWriter, r *http.Request) {
	c, e := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if e != nil {
		return
	}
	u := ident(r)
	a.hub.mu.Lock()
	a.hub.clients[c] = u.ID
	a.hub.mu.Unlock()
	defer func() {
		a.hub.mu.Lock()
		delete(a.hub.clients, c)
		a.hub.mu.Unlock()
		c.Close(websocket.StatusNormalClosure, "")
	}()
	for {
		if _, _, e = c.Read(r.Context()); e != nil {
			return
		}
	}
}
func (a *app) verifyWebhook(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	verifyToken, _ := a.metaSecrets(r.Context())
	if q.Get("hub.mode") == "subscribe" && q.Get("hub.verify_token") == verifyToken {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(q.Get("hub.challenge")))
		return
	}
	fail(w, 403, "token de verificação inválido")
}
func (a *app) receiveWebhook(w http.ResponseWriter, r *http.Request) {
	body, e := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if e != nil {
		fail(w, 400, "payload inválido")
		return
	}
	_, appSecret := a.metaSecrets(r.Context())
	if appSecret != "" {
		provided := strings.TrimPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256=")
		mac := hmac.New(sha256.New, []byte(appSecret))
		_, _ = mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(provided), []byte(expected)) {
			fail(w, 401, "assinatura do webhook inválida")
			return
		}
	}
	sum := sha256.Sum256(body)
	eventID := hex.EncodeToString(sum[:])
	tag, e := a.db.Exec(r.Context(), "INSERT INTO webhook_events(id,payload) VALUES($1,$2) ON CONFLICT DO NOTHING", eventID, body)
	if e != nil {
		fail(w, 500, "erro ao registrar evento")
		return
	}
	write(w, 200, map[string]bool{"received": true})
	if tag.RowsAffected() == 0 {
		return
	}
	go a.processWebhook(eventID, body)
}
func (a *app) processWebhook(eventID string, body []byte) {
	var p map[string]any
	if json.Unmarshal(body, &p) != nil {
		return
	}
	entries, _ := p["entry"].([]any)
	for _, er := range entries {
		entry, _ := er.(map[string]any)
		changes, _ := entry["changes"].([]any)
		for _, ch := range changes {
			change, _ := ch.(map[string]any)
			value, _ := change["value"].(map[string]any)
			metadata, _ := value["metadata"].(map[string]any)
			phoneNumberID := fmt.Sprint(metadata["phone_number_id"])
			var whatsappAccountID string
			if phoneNumberID != "<nil>" && phoneNumberID != "" {
				_ = a.db.QueryRow(context.Background(), `SELECT id FROM whatsapp_accounts WHERE phone_number_id=$1 AND active`, phoneNumberID).Scan(&whatsappAccountID)
			} else {
				_ = a.db.QueryRow(context.Background(), `SELECT id FROM whatsapp_accounts WHERE active ORDER BY created_at LIMIT 1`).Scan(&whatsappAccountID)
			}
			for _, raw := range asArray(value["messages"]) {
				m, _ := raw.(map[string]any)
				from := fmt.Sprint(m["from"])
				wid := fmt.Sprint(m["id"])
				kind := strings.ToLower(fmt.Sprint(m["type"]))
				if kind == "" || kind == "<nil>" {
					kind = "text"
				}
				text := ""
				mediaID, mimeType, filename, mediaPath := "", "", "", ""
				var mediaSize int64
				if kind == "text" {
					textMap, _ := m["text"].(map[string]any)
					text = fmt.Sprint(textMap["body"])
				} else if kind == "audio" || kind == "image" || kind == "video" || kind == "document" || kind == "sticker" {
					media, _ := m[kind].(map[string]any)
					mediaID = fmt.Sprint(media["id"])
					mimeType = fmt.Sprint(media["mime_type"])
					filename = fmt.Sprint(media["filename"])
					if filename == "<nil>" {
						filename = ""
					}
					if caption := fmt.Sprint(media["caption"]); caption != "<nil>" {
						text = caption
					}
					if text == "" {
						text = mediaLabel(kind, filename)
					}
					if mediaID != "" && mediaID != "<nil>" {
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						mediaPath, mimeType, mediaSize, _ = a.downloadIncomingMedia(ctx, whatsappAccountID, mediaID, mimeType)
						cancel()
					}
				} else {
					text = "Mensagem do tipo " + kind
				}
				var cid, mid string
				e := a.db.QueryRow(context.Background(), `SELECT c.id FROM conversations c JOIN customers cu ON cu.id=c.customer_id WHERE cu.phone=$1 AND c.whatsapp_account_id=$2 AND c.status<>'CLOSED' ORDER BY c.started_at DESC LIMIT 1`, from, whatsappAccountID).Scan(&cid)
				if e != nil {
					continue
				}
				e = a.db.QueryRow(context.Background(), `INSERT INTO messages(conversation_id,whatsapp_message_id,sender_type,type,body,status,sent_at,media_id,media_mime_type,media_filename,media_size,media_path) VALUES($1,$2,'CUSTOMER',$3,$4,'DELIVERED',now(),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,0),NULLIF($9,'')) ON CONFLICT(whatsapp_message_id) DO NOTHING RETURNING id`, cid, wid, strings.ToUpper(kind), text, mediaID, mimeType, filename, mediaSize, mediaPath).Scan(&mid)
				if e == nil {
					_, _ = a.db.Exec(context.Background(), "UPDATE conversations SET status='WAITING_AGENT',service_window_expires_at=now()+interval '24 hours',updated_at=now() WHERE id=$1", cid)
					a.hub.publish(map[string]any{"event": "message.received", "conversation_id": cid, "message_id": mid})
				}
			}
			for _, raw := range asArray(value["statuses"]) {
				s, _ := raw.(map[string]any)
				wid := fmt.Sprint(s["id"])
				status := strings.ToUpper(fmt.Sprint(s["status"]))
				if status == "SENT" || status == "DELIVERED" || status == "READ" || status == "FAILED" {
					_, _ = a.db.Exec(context.Background(), "UPDATE messages SET status=$1::message_status, delivered_at=CASE WHEN $1='DELIVERED' THEN now() ELSE delivered_at END, read_at=CASE WHEN $1='READ' THEN now() ELSE read_at END, failed_at=CASE WHEN $1='FAILED' THEN now() ELSE failed_at END WHERE whatsapp_message_id=$2", status, wid)
					a.hub.publish(map[string]any{"event": "message.status", "whatsapp_message_id": wid, "status": status})
				}
			}
		}
	}
	_, _ = a.db.Exec(context.Background(), "UPDATE webhook_events SET processed_at=now() WHERE id=$1", eventID)
}
func asArray(v any) []any { x, _ := v.([]any); return x }
func (a *app) audit(r *http.Request, userID, action, entity, id string, metadata any) {
	var eid any = nil
	if _, e := uuid.Parse(id); e == nil {
		eid = id
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	ip := r.Header.Get("X-Real-IP")
	_, _ = a.db.Exec(r.Context(), "INSERT INTO audit_logs(user_id,action,entity_type,entity_id,metadata,ip_address) VALUES(NULLIF($1,'')::uuid,$2,$3,$4,$5,NULLIF($6,'')::inet)", userID, action, entity, eid, metadata, ip)
}
