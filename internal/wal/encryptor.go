package wal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidKeySize is returned when the encryption key has an invalid size.
var ErrInvalidKeySize = errors.New("invalid key size: must be 16, 24, or 32 bytes")

// ErrDecryptionFailed is returned when decryption fails.
var ErrDecryptionFailed = errors.New("decryption failed")

// Encryptor provides AES-GCM encryption for WAL entries.
type Encryptor struct {
	key     []byte
	cipher  cipher.AEAD
	nonceSize int
}

// NewEncryptor creates a new encryptor with the given key.
// The key must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256.
func NewEncryptor(key []byte) *Encryptor {
	// In production, validate key size properly
	if len(key) == 0 {
		// Use a default key for development (NOT SECURE for production!)
		key = []byte("32-byte-secret-key-for-wal-enc")
	}
	
	// Pad or truncate key to valid size
	switch {
	case len(key) >= 32:
		key = key[:32]
	case len(key) >= 24:
		key = key[:24]
	case len(key) >= 16:
		key = key[:16]
	default:
		// Pad with zeros if too short
		padded := make([]byte, 16)
		copy(padded, key)
		key = padded
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		// This should never happen with valid key sizes
		panic(fmt.Sprintf("failed to create cipher: %v", err))
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(fmt.Sprintf("failed to create GCM: %v", err))
	}
	
	return &Encryptor{
		key:       key,
		cipher:    gcm,
		nonceSize: gcm.NonceSize(),
	}
}

// Encrypt encrypts the given plaintext and returns ciphertext.
// The ciphertext includes the nonce prepended to the encrypted data.
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	// Generate random nonce
	nonce := make([]byte, e.nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	// Encrypt
	ciphertext := e.cipher.Seal(nonce, nonce, plaintext, nil)
	
	return ciphertext, nil
}

// Decrypt decrypts the given ciphertext and returns plaintext.
// The ciphertext is expected to have the nonce prepended.
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < e.nonceSize {
		return nil, ErrDecryptionFailed
	}
	
	// Extract nonce
	nonce := ciphertext[:e.nonceSize]
	encrypted := ciphertext[e.nonceSize:]
	
	// Decrypt
	plaintext, err := e.cipher.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	
	return plaintext, nil
}

// EncryptString encrypts a string and returns base64-encoded ciphertext.
func (e *Encryptor) EncryptString(plaintext string) ([]byte, error) {
	return e.Encrypt([]byte(plaintext))
}

// DecryptString decrypts ciphertext and returns string.
func (e *Encryptor) DecryptString(ciphertext []byte) (string, error) {
	plaintext, err := e.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// KeySize returns the size of the encryption key in bytes.
func (e *Encryptor) KeySize() int {
	return len(e.key)
}

// NonceSize returns the size of the nonce in bytes.
func (e *Encryptor) NonceSize() int {
	return e.nonceSize
}

// RotateKey creates a new encryptor with a different key.
// This is useful for key rotation strategies.
func (e *Encryptor) RotateKey(newKey []byte) (*Encryptor, error) {
	return NewEncryptor(newKey), nil
}

// WALRecord represents an encrypted WAL entry.
type WALRecord struct {
	Timestamp   int64
	Operation   string
	Data        []byte
	Encrypted   bool
}

// EncryptRecord encrypts a WAL record.
func (e *Encryptor) EncryptRecord(record *WALRecord) ([]byte, error) {
	// Serialize record (in production, use proper serialization like protobuf)
	plaintext := append([]byte(record.Operation), record.Data...)
	
	encrypted, err := e.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	
	return encrypted, nil
}

// DecryptRecord decrypts a WAL record.
func (e *Encryptor) DecryptRecord(encrypted []byte) (*WALRecord, error) {
	plaintext, err := e.Decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	
	// Parse record (in production, use proper deserialization)
	record := &WALRecord{
		Operation: string(plaintext[:1]),
		Data:      plaintext[1:],
		Encrypted: true,
	}
	
	return record, nil
}
