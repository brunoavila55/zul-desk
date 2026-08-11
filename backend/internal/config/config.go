package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	AppEnv, HTTPAddr, DatabaseURL, RedisAddr, JWTSecret, CredentialEncryptionKey, CORSOrigin, UploadDir, MediaDir string
	BootstrapAdminPassword                                                                                        string
	WhatsAppAPIVersion, WhatsAppToken, WhatsAppPhoneID, WhatsAppVerifyToken, WhatsAppAppSecret                    string
	WhatsAppMock                                                                                                  bool
}

func Load() Config {
	return Config{
		AppEnv:   env("APP_ENV", "development"),
		HTTPAddr: env("HTTP_ADDR", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL"), RedisAddr: env("REDIS_ADDR", "localhost:6379"),
		JWTSecret: env("JWT_SECRET", "development-secret-change-this-now"), CORSOrigin: env("CORS_ORIGIN", "http://localhost:3000"),
		CredentialEncryptionKey: env("CREDENTIAL_ENCRYPTION_KEY", env("JWT_SECRET", "development-secret-change-this-now")),
		UploadDir:               env("UPLOAD_DIR", "./data/uploads"),
		MediaDir:                env("MEDIA_DIR", "./data/media"),
		BootstrapAdminPassword:  os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		WhatsAppAPIVersion:      env("WHATSAPP_API_VERSION", "v23.0"), WhatsAppToken: os.Getenv("WHATSAPP_ACCESS_TOKEN"),
		WhatsAppPhoneID: os.Getenv("WHATSAPP_PHONE_NUMBER_ID"), WhatsAppVerifyToken: env("WHATSAPP_VERIFY_TOKEN", "local-verify-token"),
		WhatsAppAppSecret: os.Getenv("WHATSAPP_APP_SECRET"),
		WhatsAppMock:      env("WHATSAPP_MOCK", "true") == "true",
	}
}

func Validate(cfg Config) error {
	if strings.ToLower(cfg.AppEnv) != "production" {
		return nil
	}
	if len(cfg.JWTSecret) < 32 || cfg.JWTSecret == "development-secret-change-this-now" {
		return fmt.Errorf("JWT_SECRET deve ter pelo menos 32 caracteres em produção")
	}
	if len(cfg.CredentialEncryptionKey) < 32 || cfg.CredentialEncryptionKey == cfg.JWTSecret {
		return fmt.Errorf("CREDENTIAL_ENCRYPTION_KEY deve ter pelo menos 32 caracteres e ser diferente do JWT_SECRET")
	}
	if len(cfg.BootstrapAdminPassword) < 12 || cfg.BootstrapAdminPassword == "comercial123" || strings.HasPrefix(cfg.BootstrapAdminPassword, "change-") {
		return fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD deve ter pelo menos 12 caracteres em produção")
	}
	if strings.Contains(cfg.DatabaseURL, "commercial:commercial@") {
		return fmt.Errorf("POSTGRES_PASSWORD e DATABASE_URL devem usar uma senha própria em produção")
	}
	return nil
}
func env(k, v string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return v
}
