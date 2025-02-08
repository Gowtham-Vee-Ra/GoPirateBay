package main

import (
	"crypto/sha1"
	"errors"
)

// ValidatePiece verifies if a downloaded piece matches the expected SHA-1 hash
func ValidatePiece(data []byte, expectedHash [20]byte) error {
	hash := sha1.Sum(data)
	if hash != expectedHash {
		return errors.New("piece validation failed: hashes do not match")
	}
	return nil
}
