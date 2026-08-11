package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brunoavila55/zul-desk/internal/config"
	"github.com/google/uuid"
)

type Credentials struct {
	AccessToken   string
	PhoneNumberID string
	WABAID        string
	APIVersion    string
	Mock          bool
}

type PhoneNumber struct {
	ID                 string `json:"id"`
	DisplayPhoneNumber string `json:"display_phone_number"`
	VerifiedName       string `json:"verified_name"`
	QualityRating      string `json:"quality_rating"`
	Status             string `json:"status"`
}

type Template struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Language   string `json:"language"`
	Category   string `json:"category"`
	Status     string `json:"status"`
	Components []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"components"`
}

type Client struct {
	cfg  config.Config
	http *http.Client
}

func New(cfg config.Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) SendText(ctx context.Context, credentials Credentials, phone, body string) (string, error) {
	if credentials.Mock || c.cfg.WhatsAppMock && credentials.AccessToken == "" {
		return "mock-" + uuid.NewString(), nil
	}
	payload := map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": phone, "type": "text", "text": map[string]any{"preview_url": false, "body": body}}
	return c.sendMessage(ctx, credentials, payload)
}

func (c *Client) SendTemplate(ctx context.Context, credentials Credentials, phone, name, language string, params []string) (string, error) {
	if credentials.Mock || c.cfg.WhatsAppMock && credentials.AccessToken == "" {
		return "mock-" + uuid.NewString(), nil
	}
	if language == "" {
		language = "pt_BR"
	}
	values := make([]any, 0, len(params))
	for _, v := range params {
		values = append(values, map[string]any{"type": "text", "text": v})
	}
	template := map[string]any{"name": name, "language": map[string]any{"code": language}}
	if len(values) > 0 {
		template["components"] = []any{map[string]any{"type": "body", "parameters": values}}
	}
	payload := map[string]any{"messaging_product": "whatsapp", "to": phone, "type": "template", "template": template}
	return c.sendMessage(ctx, credentials, payload)
}

func (c *Client) UploadMedia(ctx context.Context, credentials Credentials, path, mimeType, filename string) (string, error) {
	if credentials.Mock {
		return "mock-media-" + uuid.NewString(), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("messaging_product", "whatsapp")
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filepath.Base(filename)))
	header.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(part, file); err != nil {
		return "", err
	}
	if err = writer.Close(); err != nil {
		return "", err
	}
	version := credentials.APIVersion
	if version == "" {
		version = c.cfg.WhatsAppAPIVersion
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://graph.facebook.com/"+version+"/"+credentials.PhoneNumberID+"/media", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("Meta media upload retornou status %d: %s", res.StatusCode, string(raw))
	}
	var out struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("Meta não retornou o ID da mídia")
	}
	return out.ID, nil
}

func (c *Client) SendMedia(ctx context.Context, credentials Credentials, phone, kind, mediaID, caption, filename string) (string, error) {
	if credentials.Mock {
		return "mock-" + uuid.NewString(), nil
	}
	kind = strings.ToLower(kind)
	media := map[string]any{"id": mediaID}
	if caption != "" && (kind == "image" || kind == "video" || kind == "document") {
		media["caption"] = caption
	}
	if filename != "" && kind == "document" {
		media["filename"] = filename
	}
	payload := map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": phone, "type": kind, kind: media}
	return c.sendMessage(ctx, credentials, payload)
}

type DownloadedMedia struct {
	Data     []byte
	MimeType string
	FileSize int64
}

func (c *Client) DownloadMedia(ctx context.Context, credentials Credentials, mediaID string, limit int64) (DownloadedMedia, error) {
	var meta struct {
		URL      string `json:"url"`
		MimeType string `json:"mime_type"`
		FileSize int64  `json:"file_size"`
	}
	if err := c.graph(ctx, credentials, http.MethodGet, mediaID, nil, nil, &meta); err != nil {
		return DownloadedMedia{}, err
	}
	if meta.URL == "" {
		return DownloadedMedia{}, fmt.Errorf("Meta não retornou a URL da mídia")
	}
	if meta.FileSize > limit {
		return DownloadedMedia{}, fmt.Errorf("mídia excede o limite permitido")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, meta.URL, nil)
	if err != nil {
		return DownloadedMedia{}, err
	}
	req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	res, err := c.http.Do(req)
	if err != nil {
		return DownloadedMedia{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return DownloadedMedia{}, fmt.Errorf("download da mídia retornou status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return DownloadedMedia{}, err
	}
	if int64(len(data)) > limit {
		return DownloadedMedia{}, fmt.Errorf("mídia excede o limite permitido")
	}
	if meta.MimeType == "" {
		meta.MimeType = res.Header.Get("Content-Type")
	}
	return DownloadedMedia{Data: data, MimeType: meta.MimeType, FileSize: int64(len(data))}, nil
}

func MediaExtension(mimeType string) string {
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "audio/ogg": ".ogg", "audio/mpeg": ".mp3", "audio/mp4": ".m4a", "audio/aac": ".aac", "audio/amr": ".amr", "video/mp4": ".mp4", "application/pdf": ".pdf"}
	if v := ext[strings.Split(mimeType, ";")[0]]; v != "" {
		return v
	}
	return filepath.Ext(mimeType)
}

func (c *Client) sendMessage(ctx context.Context, credentials Credentials, payload map[string]any) (string, error) {
	if credentials.AccessToken == "" || credentials.PhoneNumberID == "" {
		return "", fmt.Errorf("credenciais do número não configuradas")
	}
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := c.graph(ctx, credentials, http.MethodPost, credentials.PhoneNumberID+"/messages", nil, payload, &out); err != nil {
		return "", err
	}
	if len(out.Messages) == 0 {
		return "", fmt.Errorf("Meta API não retornou o ID da mensagem")
	}
	return out.Messages[0].ID, nil
}

func (c *Client) GetPhoneNumber(ctx context.Context, credentials Credentials) (PhoneNumber, error) {
	var out PhoneNumber
	err := c.graph(ctx, credentials, http.MethodGet, credentials.PhoneNumberID, url.Values{"fields": {"id,display_phone_number,verified_name,quality_rating,status"}}, nil, &out)
	return out, err
}

func (c *Client) ListPhoneNumbers(ctx context.Context, credentials Credentials) ([]PhoneNumber, error) {
	var out struct {
		Data []PhoneNumber `json:"data"`
	}
	err := c.graph(ctx, credentials, http.MethodGet, credentials.WABAID+"/phone_numbers", url.Values{"fields": {"id,display_phone_number,verified_name,quality_rating,status"}, "limit": {"100"}}, nil, &out)
	return out.Data, err
}

func (c *Client) ListTemplates(ctx context.Context, credentials Credentials) ([]Template, error) {
	var out struct {
		Data []Template `json:"data"`
	}
	err := c.graph(ctx, credentials, http.MethodGet, credentials.WABAID+"/message_templates", url.Values{"fields": {"id,name,language,category,status,components"}, "limit": {"250"}}, nil, &out)
	return out.Data, err
}

func (c *Client) graph(ctx context.Context, credentials Credentials, method, path string, query url.Values, payload any, out any) error {
	if credentials.AccessToken == "" {
		return fmt.Errorf("access token não informado")
	}
	version := credentials.APIVersion
	if version == "" {
		version = c.cfg.WhatsAppAPIVersion
	}
	u := "https://graph.facebook.com/" + version + "/" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var body io.Reader
	if payload != nil {
		raw, _ := json.Marshal(payload)
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+credentials.AccessToken)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		var graphErr struct {
			Error struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &graphErr)
		if graphErr.Error.Message != "" {
			return fmt.Errorf("Meta API (%d): %s", graphErr.Error.Code, graphErr.Error.Message)
		}
		return fmt.Errorf("Meta API retornou status %d", res.StatusCode)
	}
	if out != nil && len(raw) > 0 {
		if err = json.Unmarshal(raw, out); err != nil {
			return err
		}
	}
	return nil
}
