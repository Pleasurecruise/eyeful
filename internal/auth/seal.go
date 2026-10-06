package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

func NewSealer(key []byte) (*Sealer, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("token key: %w", ErrTokenKeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("token key: %w", err)
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, fmt.Errorf("token cipher: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

func (s *Sealer) seal(plaintext string) []byte {
	return s.aead.Seal(nil, nil, []byte(plaintext), nil)
}

func (s *Sealer) open(sealed []byte) (string, error) {
	plaintext, err := s.aead.Open(nil, nil, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrSealedToken, err)
	}
	return string(plaintext), nil
}
