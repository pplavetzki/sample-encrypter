BUILD_DIR ?= ./bin
#acr32191.azurecr.io
ACR ?= acr32191
ENV ?= dev
AUTO_NAME = ${ACR}.azurecr.io/${ENV}

default: build

clean-bin:
	rm -rf bin/*

build:
	go build -o ${BUILD_DIR}/encrypter .

build-image: guard-TAG
	docker build -t ${AUTO_NAME}/encrypter:${TAG} --target=executable -f Dockerfile .
	docker tag ${AUTO_NAME}/encrypter:${TAG} ${AUTO_NAME}/encrypter:latest

guard-%:
	@ if [ "${${*}}" = "" ]; then \
		echo "Environment variable $* not set"; \
		exit 1; \
	fi

.PHONY: clean-bin build build-image