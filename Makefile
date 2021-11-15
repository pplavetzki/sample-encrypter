BUILD_DIR ?= ./bin

default: build

clean-bin:
	rm -rf bin/*

build:
	go build -o ${BUILD_DIR}/encrypter .

.PHONY: clean-bin build