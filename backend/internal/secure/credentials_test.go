package secure

import "testing"

func TestVaultRoundTrip(t *testing.T) {
	t.Parallel()
	vault := NewVault("a-development-key-with-enough-entropy")
	encrypted, err := vault.Encrypt("EAAB-secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "EAAB-secret-token" {
		t.Fatal("token was stored as plaintext")
	}
	decrypted, err := vault.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "EAAB-secret-token" {
		t.Fatalf("unexpected decrypted value %q", decrypted)
	}
}
