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

type LogRequest struct {
	User string `json:"user"`
}

type LoggingConfig struct {
	User     string `json:"user"`
	LogLevel string `json:"logLevel"`
}
