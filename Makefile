.PHONE: install-protoc
	brew install protobuf
	brew install protoc-gen-go
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

.PHONY: generate
generate:
	protoc --go_out=./pkg --go-grpc_out=. --go-grpc_opt=paths=source_relative pkg/proto/*.proto

.PHONY: docker-build
docker-build:
	docker build -t jemgunay/kv-store . --target runner

.PHONY: docker-up
docker-up:
	docker compose up --build