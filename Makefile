# regenerate Go pb file from .proto definitions
.PHONY: protoc
protoc:
	protoc -I proto/ proto/*.proto --go_out=plugins=grpc:proto

# delete built executables
.PHONY: clean
clean:
	rm -f client/cmd/client-tool/client-tool \
          client/cmd/generate-load/generate-load \
          client/cmd/raw-examples/raw-examples \
          server/cmd/server/server

# update vendored Go dependencies
.PHONY: update_deps
update_deps:
	go mod tidy && go mod vendor