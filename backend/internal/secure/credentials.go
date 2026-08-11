package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

type Vault struct{ key [32]byte }

func NewVault(secret string) *Vault { return &Vault{key: sha256.Sum256([]byte(secret))} }

func (v *Vault) Encrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(v.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	return "v1:" + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (v *Vault) Decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) < 3 || value[:3] != "v1:" {
		return "", fmt.Errorf("formato de credencial inválido")
	}
	raw, err := base64.RawStdEncoding.DecodeString(value[3:])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(v.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("credencial criptografada inválida")
	}
	returnValue, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	return string(returnValue), err
}
