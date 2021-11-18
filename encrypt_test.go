package main

import (
	"testing"

	types "github.com/pplavetzki/sample-encrypter/internal/types"

	"github.com/pplavetzki/sample-encrypter/mock"
)

func TestEncrypt(t *testing.T) {
	messages := []types.IncomingMessage{{Content: string(make([]byte, 200000))}}
	results := EncryptIt(messages, &mock.MockEncrypter{}, 75)
	_ = results
}
