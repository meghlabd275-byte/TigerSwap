package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Secrets Management - HashiCorp Vault, AWS KMS, GCP KMS

type Secret struct {
	Name      string
	Encrypted []byte
	Version   int
	CreatedAt int64
}

type SecretsManager struct {
	mu      sync.RWMutex
	secrets map[string]Secret
	masterKey []byte
}

func NewSecretsManager(masterKey string) (*SecretsManager, error) {
	key, err := hex.DecodeString(masterKey)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	
	return &SecretsManager{
		secrets: make(map[string]Secret),
		masterKey: key,
	}, nil
}

func (s *SecretsManager) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand, nonce); err != nil {
		return nil, err
	}
	
	return gcm.Seal(nonce, nonce, data, nil), nil
}

func (s *SecretsManager) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (s *SecretsManager) SetSecret(name string, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	encrypted, err := s.encrypt([]byte(value))
	if err != nil {
		return err
	}
	
	existing, ok := s.secrets[name]
	version := 1
	if ok {
		version = existing.Version + 1
	}
	
	s.secrets[name] = Secret{
		Name:      name,
		Encrypted: encrypted,
		Version:   version,
		CreatedAt: now(),
	}
	
	return nil
}

func (s *SecretsManager) GetSecret(name string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	secret, ok := s.secrets[name]
	if !ok {
		return "", fmt.Errorf("secret not found: %s", name)
	}
	
	decrypted, err := s.decrypt(secret.Encrypted)
	if err != nil {
		return "", err
	}
	
	return string(decrypted), nil
}

func (s *SecretsManager) DeleteSecret(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.secrets, name)
	return nil
}

func (s *SecretsManager) ListSecrets() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	names := make([]string, 0, len(s.secrets))
	for name := range s.secrets {
		names = append(names, name)
	}
	return names
}

func now() int64 {
	return int64(0) // Would use time.Now().Unix()
}

func main() {
	// Generate a 32-byte master key for demo
	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	
	manager, err := NewSecretsManager(masterKey)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	// Set secrets
	manager.SetSecret("api_key", "sk_live_1234567890")
	manager.SetSecret("private_key", "0xABCD...")
	
	// Get secret
	apiKey, _ := manager.GetSecret("api_key")
	fmt.Printf("API Key: %s\n", apiKey)
	
	// List secrets
	names := manager.ListSecrets()
	fmt.Printf("Secrets: %v\n", names)
}