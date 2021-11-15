package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	_ "expvar"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"

	helper "github.com/pplavetzki/sample-encrypter/internal"
	jose "gopkg.in/square/go-jose.v2"
)

type result struct {
	Index            int
	EncryptedMessage string
	EncryptError     error
	SignError        error
	SignedMessage    string
}

type IncomingMessage struct {
	Content string `json:"content"`
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

func EncryptIt(messages []IncomingMessage, privKey *rsa.PrivateKey, limit int) []result {

	// this buffered channel will block at the concurrency limit
	semaphoreChan := make(chan struct{}, limit)

	// this channel will not block and collect the http request results
	resultsChan := make(chan *result)

	results := make([]result, len(messages))

	defer func() {
		close(semaphoreChan)
		close(resultsChan)
	}()

	encrypter, err := getEncrypter(&privKey.PublicKey)
	if err != nil {
		panic(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.PS256, Key: privKey}, nil)
	if err != nil {
		panic(err)
	}

	for i, message := range messages {
		go func(i int, message IncomingMessage) {
			semaphoreChan <- struct{}{}
			result := &result{}
			jkweb, err := encrypter.Encrypt([]byte(message.Content))
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

func encryptHandlerFunc(privKey *rsa.PrivateKey) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var im []IncomingMessage

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

		results := EncryptIt(im, privKey, 75)
		for _, r := range results {
			if r.EncryptError != nil {
				log.Printf("encrypted value: %s\n", r.EncryptError)
			}
		}
		results = nil

		w.WriteHeader(201)
	}
}

func init() {
	go http.ListenAndServe("localhost:6060", nil)
}

func main() {
	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*90, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
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
	r.HandleFunc("/encrypt", encryptHandlerFunc(privKey)).Methods("POST")

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
