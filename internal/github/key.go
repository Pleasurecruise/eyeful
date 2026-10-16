package github

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
)

func ParseKey(value string) (*rsa.PrivateKey, error) {
	if strings.TrimSpace(value) == "" {
		return nil, ErrNoKey
	}
	encoded := []byte(value)
	if !strings.Contains(value, "BEGIN") {
		data, err := os.ReadFile(value)
		if err != nil {
			return nil, fmt.Errorf("read GitHub App private key: %w", err)
		}
		encoded = data
	}
	block, _ := pem.Decode(encoded)
	if block == nil {
		return nil, errors.New("GitHub App private key: PEM block not found")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse GitHub App private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("GitHub App private key is not RSA")
	}
	return key, nil
}
