IMAGE_NAME := kpkls-scaler
IMAGE_TAG := latest

.PHONY: proto-gen
proto-gen:
	protoc --go_out=pkg/externalscaler --go_opt=paths=source_relative \
		--go-grpc_out=pkg/externalscaler --go-grpc_opt=paths=source_relative \
		--proto_path=proto \
		externalscaler.proto

.PHONY: build
build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

.PHONY: test
test:
	go test ./...
