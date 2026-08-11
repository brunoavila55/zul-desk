package config

import "os"

type Config struct {
	HTTPAddr, DatabaseURL, RedisAddr, JWTSecret, CredentialEncryptionKey, CORSOrigin, UploadDir, MediaDir string
	WhatsAppAPIVersion, WhatsAppToken, WhatsAppPhoneID, WhatsAppVerifyToken, WhatsAppAppSecret            string
	WhatsAppMock                                                                                          bool
}

func Load() Config {
	return Config{
		HTTPAddr: env("HTTP_ADDR", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL"), RedisAddr: env("REDIS_ADDR", "localhost:6379"),
		JWTSecret: env("JWT_SECRET", "development-secret-change-this-now"), CORSOrigin: env("CORS_ORIGIN", "http://localhost:3000"),
		CredentialEncryptionKey: env("CREDENTIAL_ENCRYPTION_KEY", env("JWT_SECRET", "development-secret-change-this-now")),
		UploadDir:               env("UPLOAD_DIR", "./data/uploads"),
		MediaDir:                env("MEDIA_DIR", "./data/media"),
		WhatsAppAPIVersion:      env("WHATSAPP_API_VERSION", "v23.0"), WhatsAppToken: os.Getenv("WHATSAPP_ACCESS_TOKEN"),
		WhatsAppPhoneID: os.Getenv("WHATSAPP_PHONE_NUMBER_ID"), WhatsAppVerifyToken: env("WHATSAPP_VERIFY_TOKEN", "local-verify-token"),
		WhatsAppAppSecret: os.Getenv("WHATSAPP_APP_SECRET"),
		WhatsAppMock:      env("WHATSAPP_MOCK", "true") == "true",
	}
}
func env(k, v string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return v
}
