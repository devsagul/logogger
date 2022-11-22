package crypt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"hash"
	"os"
)

type Decryptor interface {
	Decrypt([]byte) ([]byte, error)
}

type NoOpDecryptor struct{}

func (NoOpDecryptor) Decrypt(message []byte) ([]byte, error) {
	return message, nil
}

type rsaDecryptor struct {
	privateKey *rsa.PrivateKey
	hash       hash.Hash
}

func (d rsaDecryptor) Decrypt(message []byte) ([]byte, error) {
	decrypted, err := rsa.DecryptOAEP(d.hash, rand.Reader, d.privateKey, message, nil)
	if err != nil {
		return nil, err
	}
	return decrypted, nil
}

func privateKeyFromBytes(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	bytes := block.Bytes
	// here I make an assumption that key will be in PKCS1 form, but it's not specified in the task
	return x509.ParsePKCS1PrivateKey(bytes)
}

func NewDecryptor(path string) (Decryptor, error) {
	if len(path) == 0 {
		return NoOpDecryptor{}, nil
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	privateKey, err := privateKeyFromBytes(bytes)
	if err != nil {
		return nil, err
	}

	return rsaDecryptor{
		privateKey,
		sha256.New(),
	}, nil
}
