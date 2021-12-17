package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	_ "expvar"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	helper "github.com/pplavetzki/sample-encrypter/internal"
	types "github.com/pplavetzki/sample-encrypter/internal/types"
	"github.com/pplavetzki/sample-encrypter/mock"
	jose "gopkg.in/square/go-jose.v2"
)

type JoseEncrypter struct {
	PrivateKey *rsa.PrivateKey
}

func (je *JoseEncrypter) EncryptResult(message []byte) *types.Result {

	encrypter, err := getEncrypter(&je.PrivateKey.PublicKey)
	if err != nil {
		panic(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.PS256, Key: je.PrivateKey}, nil)
	if err != nil {
		panic(err)
	}
	result := &types.Result{}
	jkweb, err := encrypter.Encrypt(message)
	if err != nil {
		result.EncryptError = err
	} else {
		result.EncryptedMessage = jkweb.FullSerialize()
		signed, err := signer.Sign([]byte(result.EncryptedMessage))
		if err != nil {
			result.SignError = err
		} else {
			result.SignedMessage = signed.FullSerialize()
		}
	}
	return result
}

func genPrivateKey() (*rsa.PrivateKey, error) {
	// The GenerateKey method takes in a reader that returns random bits, and
	// the number of bits
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

func getEncrypter(pubkey *rsa.PublicKey) (jose.Encrypter, error) {
	return jose.NewEncrypter(jose.A128GCM, jose.Recipient{Algorithm: jose.RSA_OAEP, Key: pubkey}, &jose.EncrypterOptions{})
}

func EncryptIt(messages []types.IncomingMessage, encrypter types.Encrypter, limit int) []types.Result {

	// this buffered channel will block at the concurrency limit
	semaphoreChan := make(chan struct{}, limit)

	// this channel will not block and collect the http request results
	resultsChan := make(chan *types.Result)

	results := make([]types.Result, len(messages))

	defer func() {
		close(semaphoreChan)
		close(resultsChan)
	}()

	for i, message := range messages {
		go func(i int, message types.IncomingMessage) {
			semaphoreChan <- struct{}{}
			result := encrypter.EncryptResult([]byte(message.Content))
			resultsChan <- result
			<-semaphoreChan
		}(i, message)
	}

	// start listening for any results over the resultsChan
	// once we get a result append it to the result slice
	i := 0
	for {
		res := <-resultsChan
		results[i] = *res
		i++

		// if we've reached the expected amount of urls then stop
		if i == len(messages) {
			break
		}
	}

	return results
}

func logHandlerFunc(logger *zap.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var il types.LogRequest
		var currentLogger *zap.Logger

		err := helper.DecodeJSONBody(w, r, &il)
		if err != nil {
			var mr *helper.MalformedRequest
			if errors.As(err, &mr) {
				http.Error(w, mr.Msg, mr.Status)
			} else {
				log.Println(err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		if currentLogger = helper.OverrideLogger(viper.GetString("logLevel"), il.User); currentLogger == nil {
			currentLogger = logger
		}

		currentLogger.Sugar().Debugf("user id is %s\n", il.User)
		currentLogger.Sugar().Info("user id is XXXXXXX redacted")
	}
}

func encryptHandlerFunc(encrypter types.Encrypter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var im []types.IncomingMessage

		err := helper.DecodeJSONBody(w, r, &im)
		if err != nil {
			var mr *helper.MalformedRequest
			if errors.As(err, &mr) {
				http.Error(w, mr.Msg, mr.Status)
			} else {
				log.Println(err.Error())
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		results := EncryptIt(im, encrypter, 75)
		for _, r := range results {
			if r.EncryptError != nil {
				log.Printf("encrypted value: %s\n", r.EncryptError)
			}
		}
		log.Printf("Results returned with %d messages encrypted and signed.", len(results))
		results = nil

		w.WriteHeader(201)
	}
}

func init() {
	go http.ListenAndServe("localhost:6060", nil)
}

func main() {
	var wait time.Duration
	var mockit bool
	var err error
	var cfgPath string

	cfgPath = os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "./config"
	}

	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("json")

	viper.AddConfigPath(cfgPath)
	err = viper.ReadInConfig() // Find and read the config file
	if err != nil {            // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
	viper.WatchConfig()

	logger := helper.DefaultLogger(viper.GetString("logLevel"))

	logger.Sugar().Debug("This is a debug message")

	// go watchConfig(cfgPath)

	flag.DurationVar(&wait, "graceful-timeout", time.Second*90, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.BoolVar(&mockit, "mock", false, "whether or not to block transaction or not - default false")
	flag.Parse()

	r := mux.NewRouter()

	srv := &http.Server{
		Addr: "0.0.0.0:9090",
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // Pass our instance of gorilla/mux in.
	}

	privKey, err := genPrivateKey()
	if err != nil {
		panic(err)
	}
	var encrypter types.Encrypter

	if mockit {
		log.Println("mocked")
		encrypter = &mock.MockEncrypter{}
	} else {
		encrypter = &JoseEncrypter{PrivateKey: privKey}
	}

	r.HandleFunc("/encrypt", encryptHandlerFunc(encrypter)).Methods("POST")
	r.HandleFunc("/log", logHandlerFunc(logger)).Methods("POST")

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		} else {
			log.Println("listening on :9090")
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("shutting down")
	os.Exit(0)
}
