package jobs

import (
	"encoding/json"
	"github.com/hibiken/asynq"
)

const SendMessage = "whatsapp:send_message"

type SendMessagePayload struct {
	MessageID         string   `json:"message_id"`
	ConversationID    string   `json:"conversation_id"`
	WhatsAppAccountID string   `json:"whatsapp_account_id"`
	Phone             string   `json:"phone"`
	Body              string   `json:"body"`
	Template          string   `json:"template,omitempty"`
	TemplateLanguage  string   `json:"template_language,omitempty"`
	TemplateParams    []string `json:"template_params,omitempty"`
	MediaPath         string   `json:"media_path,omitempty"`
	MediaType         string   `json:"media_type,omitempty"`
	MediaMimeType     string   `json:"media_mime_type,omitempty"`
	MediaFilename     string   `json:"media_filename,omitempty"`
}

func NewSendMessage(p SendMessagePayload) (*asynq.Task, error) {
	b, e := json.Marshal(p)
	return asynq.NewTask(SendMessage, b, asynq.MaxRetry(5)), e
}
