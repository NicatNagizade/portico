package secretbox

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

const (
	prefix = "enc:v1:"
	devKey = "portico-dev-config-key-change-me"
)

var (
	mu     sync.Mutex
	appKey = devKey
)

// SetKey keeps the application secret in memory. It is never written to the database.
// An empty key falls back to the development key.
func SetKey(key string) {
	if key == "" {
		key = devKey
	}
	mu.Lock()
	appKey = key
	mu.Unlock()
}

// UsingDevKey reports whether secrets are protected with the built-in development key.
func UsingDevKey() bool {
	return currentKey() == devKey
}

func currentKey() string {
	mu.Lock()
	defer mu.Unlock()
	return appKey
}

// Seal encrypts secret fields in a flat connection config. Other fields stay readable.
func Seal(raw []byte) ([]byte, error) {
	return apply(raw, encrypt)
}

// Open decrypts secret fields sealed by Seal. Plaintext values are left as-is.
func Open(raw []byte) ([]byte, error) {
	return apply(raw, decrypt)
}

// Redact clears secret fields so API responses do not return them.
func Redact(raw []byte) ([]byte, error) {
	return apply(raw, func(string) (string, error) { return "", nil })
}

// NeedsSeal reports whether raw still has a plaintext secret.
func NeedsSeal(raw []byte) bool {
	cfg, err := parse(raw)
	if err != nil {
		return false
	}
	for key, value := range cfg {
		text, ok := value.(string)
		if ok && isSecret(key) && text != "" && !strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// Merge keeps stored secrets when an update leaves them blank.
func Merge(existing, incoming []byte) ([]byte, error) {
	oldCfg, err := parse(existing)
	if err != nil {
		return nil, err
	}
	newCfg, err := parse(incoming)
	if err != nil {
		return nil, err
	}
	for key, value := range newCfg {
		text, isText := value.(string)
		if isSecret(key) && isText && text == "" {
			if prev, ok := oldCfg[key]; ok {
				newCfg[key] = prev
			}
		}
	}
	for key, value := range oldCfg {
		if _, ok := newCfg[key]; !ok && isSecret(key) {
			newCfg[key] = value
		}
	}
	return json.Marshal(newCfg)
}

func apply(raw []byte, fn func(string) (string, error)) ([]byte, error) {
	cfg, err := parse(raw)
	if err != nil {
		return nil, err
	}
	for key, value := range cfg {
		text, ok := value.(string)
		if !ok || !isSecret(key) {
			continue
		}
		next, err := fn(text)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		cfg[key] = next
	}
	return json.Marshal(cfg)
}

func parse(raw []byte) (map[string]any, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return map[string]any{}, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if cfg == nil {
		return map[string]any{}, nil
	}
	return cfg, nil
}

func isSecret(key string) bool {
	switch strings.ToLower(strings.ReplaceAll(key, "-", "_")) {
	case "password", "passwd", "api_key", "apikey", "token", "secret", "dsn", "uri":
		return true
	default:
		return false
	}
}

func encrypt(plain string) (string, error) {
	if plain == "" || strings.HasPrefix(plain, prefix) {
		return plain, nil
	}
	aead, err := gcm()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(plain), nil)
	return prefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, prefix) {
		return value, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", err
	}
	aead, err := gcm()
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", fmt.Errorf("sealed value is too short")
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func gcm() (cipher.AEAD, error) {
	sum := sha256.Sum256([]byte(currentKey()))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
