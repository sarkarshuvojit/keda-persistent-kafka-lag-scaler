IMAGE_NAME := kpkls
IMAGE_TAG := latest

.PHONY: build
build:
	eval $$(minikube docker-env) && docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

COUNT ?= 100
SIZE ?= 64

.PHONY: load
load:
	kubectl run curl-load --rm -it --restart=Never --image=curlimages/curl -- \
		curl -s -X POST http://kafka-producer/produce \
		-H 'Content-Type: application/json' \
		-d '{"count":$(COUNT),"messageSize":$(SIZE)}'
