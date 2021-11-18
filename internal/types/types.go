package types

type Result struct {
	Index            int
	EncryptedMessage string
	EncryptError     error
	SignError        error
	SignedMessage    string
}

type IncomingMessage struct {
	Content string `json:"content"`
}

type Encrypter interface {
	EncryptResult([]byte) *Result
}
