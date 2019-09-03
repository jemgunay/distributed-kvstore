[![CircleCI](https://circleci.com/gh/jemgunay/distributed-kvstore/tree/master.svg?style=svg)](https://circleci.com/gh/jemgunay/distributed-kvstore/tree/master)

# Distributed gRPC Key-Value Store

A basic eventually consistent distributed key-value store and gRPC server written in Golang. Supports the storage of arbitrary byte values and resolves sync conflicts across nodes. Client examples are also included.

## Creating Server Nodes

Use the `spawn.sh` script to create N number of nodes at once, linking each of them via a list of `node_address` startup flags:
```bash
cd server/cmd/server
# Create and link 3 nodes (serving on ports 7001-7003).
./spawn.sh 3
```

Manually creating a single node (not linked to any other nodes):
```bash
cd server/cmd/server
go build && ./server -port=7001
```

Manually creating nodes (linked to other nodes) - provide multiple `node_address` flags, one for each node that this new instance should sync with:
```bash
cd server/cmd/server
go build && ./server -port=7001 -node_address=":7002" -node_address=":7003"
```

## Connect to Nodes via a Command-Line Client

```bash
cd client/cmd/client-tool
go build && ./client-tool -port=7001
```

### Example Tool Input

`publish animals dog`<br>
`fetch animals`<br>
`delete animals`

## Programmatically Connect to Nodes in Go

See `client/cmd/raw-examples/main.go` for examples on how to create a client and perform publish, fetch and delete operations within a Go service consuming the client package.