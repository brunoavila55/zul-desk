package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/brunoavila55/zul-desk/internal/config"
	"github.com/brunoavila55/zul-desk/internal/jobs"
	"github.com/brunoavila55/zul-desk/internal/secure"
	"github.com/brunoavila55/zul-desk/internal/whatsapp"
)

func main() {
	cfg := config.Load()
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database_connection_failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	wa := whatsapp.New(cfg)
	vault := secure.NewVault(cfg.CredentialEncryptionKey)
	server := asynq.NewServer(asynq.RedisClientOpt{Addr: cfg.RedisAddr}, asynq.Config{Concurrency: 10, Queues: map[string]int{"critical": 6, "default": 4}})
	mux := asynq.NewServeMux()
	mux.HandleFunc(jobs.SendMessage, func(ctx context.Context, t *asynq.Task) error {
		var p jobs.SendMessagePayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return err
		}
		_, _ = db.Exec(ctx, "UPDATE messages SET status='QUEUED' WHERE id=$1", p.MessageID)
		var encryptedToken, phoneNumberID, wabaID, apiVersion, onboardingType string
		if err := db.QueryRow(ctx, `SELECT access_token_encrypted,phone_number_id,business_account_id,api_version,onboarding_type FROM whatsapp_accounts WHERE id=$1 AND active`, p.WhatsAppAccountID).Scan(&encryptedToken, &phoneNumberID, &wabaID, &apiVersion, &onboardingType); err != nil {
			_, _ = db.Exec(ctx, "UPDATE messages SET status='FAILED',failed_at=now(),error_message='Número do WhatsApp indisponível' WHERE id=$1", p.MessageID)
			return err
		}
		token, err := vault.Decrypt(encryptedToken)
		if err != nil {
			return err
		}
		credentials := whatsapp.Credentials{AccessToken: token, PhoneNumberID: phoneNumberID, WABAID: wabaID, APIVersion: apiVersion, Mock: onboardingType == "DEMO"}
		var wid string
		if p.MediaPath != "" {
			mediaID, uploadErr := wa.UploadMedia(ctx, credentials, p.MediaPath, p.MediaMimeType, p.MediaFilename)
			if uploadErr != nil {
				err = uploadErr
			} else {
				wid, err = wa.SendMedia(ctx, credentials, p.Phone, p.MediaType, mediaID, p.Body, p.MediaFilename)
				if err == nil {
					_, _ = db.Exec(ctx, "UPDATE messages SET media_id=$2 WHERE id=$1", p.MessageID, mediaID)
				}
			}
		} else if p.Template != "" {
			wid, err = wa.SendTemplate(ctx, credentials, p.Phone, p.Template, p.TemplateParams)
		} else {
			wid, err = wa.SendText(ctx, credentials, p.Phone, p.Body)
		}
		if err != nil {
			_, _ = db.Exec(ctx, "UPDATE messages SET status='FAILED',failed_at=now(),error_message=$2 WHERE id=$1", p.MessageID, err.Error())
			return err
		}
		_, err = db.Exec(ctx, "UPDATE messages SET status='SENT',whatsapp_message_id=$2,sent_at=$3 WHERE id=$1", p.MessageID, wid, time.Now())
		log.Info("whatsapp_message_sent", "message_id", p.MessageID, "conversation_id", p.ConversationID)
		return err
	})
	log.Info("worker_started")
	if err := server.Run(mux); err != nil {
		log.Error("worker_stopped", "error", err)
	}
}
