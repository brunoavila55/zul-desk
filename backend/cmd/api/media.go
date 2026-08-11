package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/brunoavila55/zul-desk/internal/jobs"
	"github.com/brunoavila55/zul-desk/internal/whatsapp"
)

const maxMediaBytes int64 = 25 << 20

func (a *app) sendMedia(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	cid := chi.URLParam(r, "id")
	r.Body = http.MaxBytesReader(w, r.Body, maxMediaBytes+(2<<20))
	if err := r.ParseMultipartForm(maxMediaBytes); err != nil {
		fail(w, 400, "arquivo inválido ou maior que 25 MB")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, 400, "selecione um arquivo")
		return
	}
	defer file.Close()
	first := make([]byte, 512)
	n, _ := io.ReadFull(file, first)
	first = first[:n]
	mimeType := http.DetectContentType(first)
	if declared := header.Header.Get("Content-Type"); declared != "" && mimeType == "application/octet-stream" {
		mimeType = strings.Split(declared, ";")[0]
	}
	if declared := strings.Split(header.Header.Get("Content-Type"), ";")[0]; mimeType == "application/ogg" && strings.HasPrefix(declared, "audio/") {
		mimeType = declared
	}
	kind := strings.ToLower(strings.TrimSpace(r.FormValue("kind")))
	if kind == "" {
		kind = mediaKind(mimeType)
	}
	if !validMedia(kind, mimeType) {
		fail(w, 415, "formato de mídia não suportado")
		return
	}
	var phone, status, accountID string
	var expires *time.Time
	err = a.db.QueryRow(r.Context(), `SELECT cu.phone,c.status::text,c.whatsapp_account_id,c.service_window_expires_at FROM conversations c JOIN customers cu ON cu.id=c.customer_id WHERE c.id=$1 AND c.assigned_user_id=$2`, cid, u.ID).Scan(&phone, &status, &accountID, &expires)
	if err != nil {
		fail(w, 403, "atendimento não encontrado ou atribuído a outro usuário")
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
	ext := whatsapp.MediaExtension(mimeType)
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(header.Filename))
	}
	name := uuid.NewString() + ext
	path := filepath.Join(a.cfg.MediaDir, name)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		fail(w, 500, "não foi possível armazenar a mídia")
		return
	}
	_, _ = out.Write(first)
	_, err = io.Copy(out, file)
	closeErr := out.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(path)
		fail(w, 500, "não foi possível armazenar a mídia")
		return
	}
	if kind == "audio" && (strings.HasPrefix(mimeType, "audio/webm") || strings.HasPrefix(mimeType, "video/webm")) {
		converted := filepath.Join(a.cfg.MediaDir, uuid.NewString()+".ogg")
		cmd := exec.CommandContext(r.Context(), "ffmpeg", "-y", "-i", path, "-vn", "-c:a", "libopus", "-b:a", "32k", converted)
		if output, e := cmd.CombinedOutput(); e == nil {
			_ = os.Remove(path)
			path = converted
			mimeType = "audio/ogg"
			name = filepath.Base(converted)
		} else {
			a.log.Warn("audio_conversion_failed", "error", e, "output", string(output))
		}
	}
	info, _ := os.Stat(path)
	filename := filepath.Base(header.Filename)
	caption := strings.TrimSpace(r.FormValue("caption"))
	body := caption
	if body == "" {
		body = mediaLabel(kind, filename)
	}
	var mid string
	err = a.db.QueryRow(r.Context(), `INSERT INTO messages(conversation_id,sender_type,user_id,type,body,status,media_mime_type,media_filename,media_size,media_path) VALUES($1,'AGENT',$2,$3,$4,'PENDING',$5,$6,$7,$8) RETURNING id`, cid, u.ID, strings.ToUpper(kind), body, mimeType, filename, info.Size(), path).Scan(&mid)
	if err != nil {
		_ = os.Remove(path)
		fail(w, 500, "não foi possível registrar a mídia")
		return
	}
	_, _ = a.db.Exec(r.Context(), "UPDATE conversations SET status='WAITING_CUSTOMER',updated_at=now() WHERE id=$1", cid)
	task, _ := jobs.NewSendMessage(jobs.SendMessagePayload{MessageID: mid, ConversationID: cid, WhatsAppAccountID: accountID, Phone: phone, Body: caption, MediaPath: path, MediaType: kind, MediaMimeType: mimeType, MediaFilename: filename})
	if _, err = a.queue.Enqueue(task); err != nil {
		_, _ = a.db.Exec(r.Context(), "UPDATE messages SET status='FAILED',error_message='fila indisponível' WHERE id=$1", mid)
	}
	write(w, 201, map[string]any{"id": mid, "type": strings.ToUpper(kind), "body": body, "media_url": "/messages/" + mid + "/media"})
}

func (a *app) serveMessageMedia(w http.ResponseWriter, r *http.Request) {
	u := ident(r)
	id := chi.URLParam(r, "id")
	var path, mimeType, filename string
	var size int64
	err := a.db.QueryRow(r.Context(), `SELECT m.media_path,COALESCE(m.media_mime_type,'application/octet-stream'),COALESCE(m.media_filename,'media'),COALESCE(m.media_size,0) FROM messages m JOIN conversations c ON c.id=m.conversation_id WHERE m.id=$1 AND (c.assigned_user_id=$2 OR $3<>'AGENT')`, id, u.ID, u.Role).Scan(&path, &mimeType, &filename, &size)
	if err != nil || path == "" {
		fail(w, 404, "mídia não encontrada")
		return
	}
	file, err := os.Open(path)
	if err != nil {
		fail(w, 404, "arquivo de mídia indisponível")
		return
	}
	defer file.Close()
	if info, e := file.Stat(); e == nil {
		size = info.Size()
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", strings.ReplaceAll(filename, "\"", "")))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, filename, time.Now(), file)
}

func mediaKind(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	default:
		return "document"
	}
}
func validMedia(kind, mimeType string) bool {
	if kind == "sticker" {
		return mimeType == "image/webp"
	}
	if kind == "image" {
		return mimeType == "image/jpeg" || mimeType == "image/png" || mimeType == "image/webp"
	}
	if kind == "audio" {
		return strings.HasPrefix(mimeType, "audio/") || mimeType == "video/webm"
	}
	if kind == "video" {
		return mimeType == "video/mp4"
	}
	if kind == "document" {
		return mimeType != "application/x-msdownload"
	}
	return false
}
func mediaLabel(kind, filename string) string {
	switch kind {
	case "audio":
		return "Áudio"
	case "image":
		return "Imagem"
	case "video":
		return "Vídeo"
	case "sticker":
		return "Figurinha"
	default:
		if filename != "" {
			return filename
		}
		return "Documento"
	}
}

func (a *app) downloadIncomingMedia(ctx context.Context, accountID, mediaID, declaredMime string) (string, string, int64, error) {
	credentials, err := a.accountCredentials(ctx, accountID)
	if err != nil {
		return "", declaredMime, 0, err
	}
	if credentials.Mock {
		return "", declaredMime, 0, nil
	}
	downloaded, err := a.wa.DownloadMedia(ctx, credentials, mediaID, maxMediaBytes)
	if err != nil {
		return "", declaredMime, 0, err
	}
	mimeType := downloaded.MimeType
	if mimeType == "" {
		mimeType = declaredMime
	}
	name := uuid.NewString() + whatsapp.MediaExtension(mimeType)
	path := filepath.Join(a.cfg.MediaDir, name)
	if err = os.WriteFile(path, downloaded.Data, 0600); err != nil {
		return "", mimeType, 0, err
	}
	return path, mimeType, downloaded.FileSize, nil
}
