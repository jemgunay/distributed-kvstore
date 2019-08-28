protoc:
	protoc -I proto/ proto/kv.proto --go_out=plugins=grpc:proto

clean:
	rm client/client
	rm server/server