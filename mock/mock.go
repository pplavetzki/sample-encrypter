package mock

import (
	"time"

	types "github.com/pplavetzki/sample-encrypter/internal/types"
)

type MockEncrypter struct {
}

func makeBuffer() []byte {
	return make([]byte, 5000)
}

func (m *MockEncrypter) EncryptResult(incomingMessage []byte) *types.Result {
	time.Sleep(1 * time.Second)
	return &types.Result{
		Index:            0,
		EncryptedMessage: string(makeBuffer()),
		SignedMessage:    string(makeBuffer()),
	}
}
