package config

import "testing"

func TestValidateProductionSecrets(t *testing.T) {
	t.Parallel()
	bad := Config{AppEnv: "production", JWTSecret: "short", CredentialEncryptionKey: "short", BootstrapAdminPassword: "short"}
	if Validate(bad) == nil {
		t.Fatal("expected production validation error")
	}
	good := Config{AppEnv: "production", JWTSecret: "12345678901234567890123456789012", CredentialEncryptionKey: "abcdefghijklmnopqrstuvwxyz123456", BootstrapAdminPassword: "strong-admin-password"}
	if err := Validate(good); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateRejectsDefaultDatabasePassword(t *testing.T) {
	t.Parallel()
	cfg := Config{
		AppEnv: "production", JWTSecret: "12345678901234567890123456789012",
		CredentialEncryptionKey: "abcdefghijklmnopqrstuvwxyz123456",
		BootstrapAdminPassword:  "strong-admin-password",
		DatabaseURL:             "postgres://commercial:commercial@postgres:5432/commercial?sslmode=disable",
	}
	if Validate(cfg) == nil {
		t.Fatal("expected default database password to be rejected")
	}
}
