package main

import (
	"crypto/rand"
	"errors"
)

var pwletters string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ012345678a9!$"

func makepassword() (string, error) {
	b := make([]byte, 16)
	ln, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	if ln != 16 {
		return "", errors.New("failed to read random data")
	}

	for c := range b {
		b[c] = pwletters[b[c]&0x3F]
	}

	return string(b), nil
}
