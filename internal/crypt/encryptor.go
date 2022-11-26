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

type Encryptor interface {
	Encrypt([]byte) ([]byte, error)
}

type noopEncryptor struct{}

func (noopEncryptor) Encrypt(message []byte) ([]byte, error) {
	return message, nil
}

type rsaEncryptor struct {
	publicKey *rsa.PublicKey
	hash      hash.Hash
}

func (e rsaEncryptor) Encrypt(message []byte) ([]byte, error) {
	encrypted, err := rsa.EncryptOAEP(e.hash, rand.Reader, e.publicKey, message, nil)
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

func publicKeyFromBytes(raw []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(raw)
	bytes := block.Bytes
	// here I make an assumption that key will be in PKCS1 form, but it's not specified in the task
	return x509.ParsePKCS1PublicKey(bytes)
}

func NewEncryptor(path string) (Encryptor, error) {
	if len(path) == 0 {
		return noopEncryptor{}, nil
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	publicKey, err := publicKeyFromBytes(bytes)
	if err != nil {
		return nil, err
	}

	return rsaEncryptor{
		publicKey,
		sha256.New(),
	}, nil
}
