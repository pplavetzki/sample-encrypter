package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/golang/gddo/httputil/header"
	"github.com/pplavetzki/sample-encrypter/internal/types"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type MalformedRequest struct {
	Status int
	Msg    string
}

func (mr *MalformedRequest) Error() string {
	return mr.Msg
}

func DecodeJson(data io.ReadCloser, dst interface{}) error {
	dec := json.NewDecoder(data)
	dec.DisallowUnknownFields()

	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return &MalformedRequest{Status: http.StatusBadRequest, Msg: msg}

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := fmt.Sprintf("Request body contains badly-formed JSON")
			return &MalformedRequest{Status: http.StatusBadRequest, Msg: msg}

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return &MalformedRequest{Status: http.StatusBadRequest, Msg: msg}

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return &MalformedRequest{Status: http.StatusBadRequest, Msg: msg}

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return &MalformedRequest{Status: http.StatusBadRequest, Msg: msg}

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			return &MalformedRequest{Status: http.StatusRequestEntityTooLarge, Msg: msg}

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return &MalformedRequest{Status: http.StatusBadRequest, Msg: msg}
	}

	return nil
}

func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	if r.Header.Get("Content-Type") != "" {
		value, _ := header.ParseValueAndParams(r.Header, "Content-Type")
		if value != "application/json" {
			msg := "Content-Type header is not application/json"
			return &MalformedRequest{Status: http.StatusUnsupportedMediaType, Msg: msg}
		}
	}

	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	return DecodeJson(r.Body, dst)
}

func ReadConfig(path string) (types.LoggingConfig, error) {
	var cfg types.LoggingConfig
	cfMap, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(cfMap, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func DefaultLogger(logLevel string) *zap.Logger {
	if logLevel == "" {
		logLevel = "info"
	}
	rawJSON := []byte(`{
		"level": "` + logLevel + `",
		"encoding": "json",
		"outputPaths": ["stdout"],
		"errorOutputPaths": ["stderr"],
		"initialFields": {"foo": "bar"},
		"encoderConfig": {
		  "messageKey": "message",
		  "levelKey": "level",
		  "levelEncoder": "lowercase"
		}
	  }`)

	var cfg zap.Config
	if err := json.Unmarshal(rawJSON, &cfg); err != nil {
		panic(err)
	}
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	return logger
}

func OverrideLogger(logLevel, userName string) *zap.Logger {
	over := viper.GetStringMapString("overrides")
	if over == nil {
		return nil
	}
	if over["user"] != userName {
		return nil
	}
	if over["loglevel"] == logLevel || over["loglevel"] == "" {
		return nil
	}
	return DefaultLogger(over["loglevel"])
}
