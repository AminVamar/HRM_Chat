package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

const keyLength = 32 // AES-256

// defaultKey нужен, чтобы проект запускался без настройки.
// С ним сообщения фактически не защищены, поэтому Init об этом сообщает.
var defaultKey = []byte("12345678901234567890123456789012")

var secretKey = defaultKey

// ErrDecryptFailed — не удалось расшифровать сообщение.
// Обычно это значит, что сменили ENCRYPTION_KEY или запись в базе повреждена.
var ErrDecryptFailed = errors.New("failed to decrypt content")

// Init загружает ключ шифрования из ENCRYPTION_KEY.
// Если длина ключа неправильная — возвращает ошибку.
// Возвращает true, если используется встроенный ключ по умолчанию.
func Init() (usingDefaultKey bool, err error) {
	keyEnv := os.Getenv("ENCRYPTION_KEY")
	if keyEnv == "" {
		secretKey = defaultKey
		return true, nil
	}
	if len(keyEnv) != keyLength {
		return false, fmt.Errorf("ENCRYPTION_KEY must be exactly %d bytes, got %d", keyLength, len(keyEnv))
	}
	secretKey = []byte(keyEnv)
	return false, nil
}

func Encrypt(text string) (string, error) {
	if text == "" {
		return "", nil
	}

	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt расшифровывает текст.
// Старые сообщения, сохранённые без шифрования, возвращаются как есть.
// Если текст похож на шифр, но не расшифровывается — возвращается ErrDecryptFailed.
func Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return encoded, nil // старый текст без шифрования
	}

	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(data) < gcm.NonceSize()+gcm.Overhead() {
		return encoded, nil // слишком короткий для шифра — старый текст
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptFailed
	}

	return string(plaintext), nil
}
