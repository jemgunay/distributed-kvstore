protoc:
	protoc -I proto/ proto/kv.proto --go_out=plugins=grpc:proto