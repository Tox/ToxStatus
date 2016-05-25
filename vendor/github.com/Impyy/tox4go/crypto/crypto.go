package crypto

import (
	"crypto/rand"
	"encoding/binary"
	"errors"

	"golang.org/x/crypto/nacl/box"
)

// Encrypt uses a crypto_box_afternm-equivalent function to encrypt the given data.
func Encrypt(data []byte, sharedKey *[SharedKeySize]byte) ([]byte, *[NonceSize]byte, error) {
	nonce, err := GenerateNonce()
	if err != nil {
		return nil, nil, err
	}

	return box.SealAfterPrecomputation(nil, data, nonce, sharedKey), nonce, nil
}

// Decrypt uses a crypto_box_open_afternm-equivalent function to decrypt the given data.
func Decrypt(encryptedData []byte, sharedKey *[SharedKeySize]byte, nonce *[NonceSize]byte) ([]byte, error) {
	data, success := box.OpenAfterPrecomputation(nil, encryptedData, nonce, sharedKey)
	if !success {
		return nil, errors.New("decryption failed")
	}

	return data, nil
}

// GenerateNonce generates a random nonce.
func GenerateNonce() (*[NonceSize]byte, error) {
	nonce := new([NonceSize]byte)

	_, err := rand.Read(nonce[:])
	if err != nil {
		return nil, err
	}

	return nonce, nil
}

// GeneratePingID generates a new random ping ID.
func GeneratePingID() (uint64, error) {
	pingID := new([8]byte)

	_, err := rand.Read(pingID[:])
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(pingID[:]), nil
}

// PrecomputeKey calculates the shared key between the peer's publicKey and our own secret key.
func PrecomputeKey(publicKey *[PublicKeySize]byte, secretKey *[SecretKeySize]byte) *[SharedKeySize]byte {
	sharedKey := new([SharedKeySize]byte)
	box.Precompute(sharedKey, publicKey, secretKey)

	return sharedKey
}

// GenerateKeyPair generates a new curve25519 keypair.
func GenerateKeyPair() (*[PublicKeySize]byte, *[SecretKeySize]byte, error) {
	return box.GenerateKey(rand.Reader)
}

// Zero replaces all values in the given buffer with zeros.
// This might give a false sense of security.
// There is no guarantee that the system hasn't swapped a piece of RAM to disk that contains this buffer.
func Zero(buffer []byte) {
	copy(buffer, make([]byte, len(buffer)))
}
