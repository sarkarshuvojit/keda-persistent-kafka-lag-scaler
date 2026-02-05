IMAGE_NAME := kpkls
IMAGE_TAG := latest

.PHONY: build
build:
	eval $$(minikube docker-env) && docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
