package crypto

import (
	"errors"
	"strings"
	"testing"
)

func TestAES256GCMEncryptDecrypt(t *testing.T) {
	tests := []string{
		"Hello World!",
		"Привет Амин, Фед и Эрнест!",
		"AES-256-GCM Secure Encryption 2026",
		"Special chars: @#$%^&*()_+~`|}{[]:;?><,./-=",
	}

	for _, original := range tests {
		encrypted, err := Encrypt(original)
		if err != nil {
			t.Fatalf("Encrypt failed for '%s': %v", original, err)
		}
		if encrypted == original {
			t.Errorf("Expected encrypted string to differ from original '%s'", original)
		}

		decrypted, err := Decrypt(encrypted)
		if err != nil {
			t.Fatalf("Decrypt failed for '%s': %v", original, err)
		}

		if decrypted != original {
			t.Errorf("Mismatch for '%s': got '%s', expected '%s'", original, decrypted, original)
		}
	}
}

func TestEncryptEmptyStaysEmpty(t *testing.T) {
	encrypted, err := Encrypt("")
	if err != nil || encrypted != "" {
		t.Fatalf("Encrypt(\"\") = %q, %v; want \"\", nil", encrypted, err)
	}

	decrypted, err := Decrypt("")
	if err != nil || decrypted != "" {
		t.Fatalf("Decrypt(\"\") = %q, %v; want \"\", nil", decrypted, err)
	}
}

// Старые сообщения без шифрования должны читаться.
func TestDecryptPassesThroughLegacyPlainText(t *testing.T) {
	legacy := []string{
		"Привет",                    // не base64
		"test",                      // base64, но слишком короткий для шифра
		"Hello world, how are you?", // не base64
	}

	for _, original := range legacy {
		got, err := Decrypt(original)
		if err != nil {
			t.Errorf("Decrypt(%q) returned error %v; legacy text should pass through", original, err)
		}
		if got != original {
			t.Errorf("Decrypt(%q) = %q; want unchanged", original, got)
		}
	}
}

// Испорченный шифр должен вернуть ошибку, а не мусор в base64.
func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	encrypted, err := Encrypt("secret message")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Меняем один символ, чтобы base64 остался валидным.
	tampered := []byte(encrypted)
	if tampered[len(tampered)-3] == 'A' {
		tampered[len(tampered)-3] = 'B'
	} else {
		tampered[len(tampered)-3] = 'A'
	}

	if _, err := Decrypt(string(tampered)); !errors.Is(err, ErrDecryptFailed) {
		t.Errorf("Decrypt(tampered) error = %v; want ErrDecryptFailed", err)
	}
}

func TestInitRejectsWrongKeyLength(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "too-short")

	if _, err := Init(); err == nil {
		t.Fatal("Init accepted a key that is not 32 bytes long")
	} else if !strings.Contains(err.Error(), "32") {
		t.Errorf("Init error = %v; want it to mention the required length", err)
	}
}

func TestInitReportsDefaultKey(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", "")

	usingDefault, err := Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if !usingDefault {
		t.Error("Init did not report that the insecure built-in key is in use")
	}
}
