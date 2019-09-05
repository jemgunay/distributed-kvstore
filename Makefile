# regenerate Go pb file from .proto definitions
protoc:
	protoc -I proto/ proto/*.proto --go_out=plugins=grpc:proto

# delete built executables
clean:
	rm -f client/cmd/client-tool/client-tool \
          client/cmd/generate-load/generate-load \
          client/cmd/raw-examples/raw-examples \
          server/cmd/server/server
