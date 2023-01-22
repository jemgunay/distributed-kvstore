.PHONE: install-protoc
	brew install protobuf
	brew install protoc-gen-go
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2


.PHONY: generate
generate:
	#protoc -I proto/ proto/*.proto --go_out=plugins=grpc:proto
	#protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative helloworld/helloworld.proto
	protoc --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/*.proto

