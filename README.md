# Distributed gRPC Key-Value Store

[![CircleCI](https://dl.circleci.com/status-badge/img/gh/jemgunay/distributed-kvstore/tree/master.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/jemgunay/distributed-kvstore/tree/master)

A basic eventually-consistent distributed key-value store written in Golang. 
Supports the storage of arbitrary byte values and resolves sync conflicts via message timestamp across nodes. 
Utilises gRPC and protocol buffers to expose a performant language-agnostic interface. Multiple client examples are also included.

## Create Cluster

### Docker

```shell
make docker-build
# Spawns a 3-node cluster
make docker-up
```

### Manual Script

<details>
<summary>Click here for usage details</summary>

Use the `spawn.sh` script to create N number of nodes at once, linking each of them via a list of `node_address` startup flags:
```shell
# Create and automatically link 3 nodes (serving on ports 7001-7003).
./cmd/server/spawn.sh 3
# Manually creating a single node (not linked to any other nodes):
go run cmd/server/main.go -port=7001
# Manually creating nodes (linked to other nodes)
go run cmd/server/main.go -port=7001 -node_address=":7002" -node_address=":7003"
```
</details>

## Connecting to Nodes

### CLI Client

```shell
go run cmd/client/cli/main.go -addr="localhost:7001"
# Example input for client-tool
publish animals dog
fetch animals
delete animals
subscribe animals
```

### Load Test Tool

```shell
go run cmd/client/generate-load/main.go -num_services=3
```

### Programmatically

See `cmd/client/raw-examples/main.go` for examples on how to create a client and perform publish, fetch and delete operations within a Go service consuming the client package.
