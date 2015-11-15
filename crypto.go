//partially taken from https://github.com/Impyy/toxing.me/blob/master/crypto.go, also licensed under AGPL

package main

import (
	"crypto/rand"
	"fmt"

	"github.com/GoKillers/libsodium-go/cryptobox"
)

type Crypto struct {
	PublicKey []byte
	SecretKey []byte
}

func NewCrypto() (*Crypto, error) {
	secretKey, publicKey, _ := cryptobox.CryptoBoxKeyPair()
	return NewCryptoFrom(publicKey, secretKey)
}

func NewCryptoFrom(publicKey []byte, secretKey []byte) (*Crypto, error) {
	publicLen := cryptobox.CryptoBoxPublicKeyBytes()
	secretLen := cryptobox.CryptoBoxSecretKeyBytes()

	if len(publicKey) != publicLen {
		return nil, fmt.Errorf("public key must be exactly %d bytes long", publicLen)
	}

	if len(secretKey) != secretLen {
		return nil, fmt.Errorf("secret key must be exactly %d bytes long", secretLen)
	}

	return &Crypto{publicKey, secretKey}, nil
}

func encryptData(data []byte, secretKey []byte, nonce []byte) []byte {
	padding := cryptobox.CryptoBoxZeroBytes()

	tempData := make([]byte, len(data)+padding)
	for i := 0; i < len(data); i++ {
		tempData[i+padding] = data[i]
	}

	encrypted, result := cryptobox.CryptoBoxAfterNm(tempData, nonce, secretKey)

	if encrypted == nil || result != 0 {
		return nil
	}

	return encrypted
}

func decryptData(data []byte, secretKey []byte, nonce []byte) []byte {
	padding := cryptobox.CryptoBoxBoxZeroBytes()

	tempData := make([]byte, len(data)+padding)
	for i := 0; i < len(data); i++ {
		tempData[i+padding] = data[i]
	}

	decrypted, result := cryptobox.CryptoBoxOpenAfterNm(tempData, nonce, secretKey)

	if decrypted == nil || result != 0 {
		return nil
	}

	return decrypted
}

func nextNonce() []byte {
	return nextBytes(cryptobox.CryptoBoxNonceBytes())
}

func nextBytes(amount int) []byte {
	bytes := make([]byte, amount)
	rand.Read(bytes)
	return bytes
}

func (c *Crypto) CreateSharedKey(publicKey []byte) []byte {
	sharedKey, result := cryptobox.CryptoBoxBeforeNm(publicKey, c.SecretKey)

	if result == 0 {
		return sharedKey
	}

	return nil
}

func generateKeyPair() ([]byte, []byte, int) {
	return cryptobox.CryptoBoxKeyPair()
}
