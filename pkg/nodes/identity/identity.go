package identity

import (
	"fmt"
	"net"
	"strconv"
	"time"

	petname "github.com/dustinkirkland/golang-petname"

	pb "github.com/jemgunay/distributed-kvstore/pkg/proto"
)

func init() {
	// seed random name generation
	petname.NonDeterministicMode()
}

// Identity represents a node's identity.
type Identity struct {
	StartTime            int64
	Address              string
	Name                 string
	ID                   string
	LastMessageTimestamp int64 // TODO: implement this
}

// New initialises a new Identity with a randomised name.
func New(port int) (Identity, error) {
	name := petname.Generate(3, "-")
	startTime := time.Now().UTC().UnixNano()

	hostname, err := getPrivateIP()
	if err != nil {
		return Identity{}, fmt.Errorf("attempting to lookup local IP address: %w", err)
	}
	address := net.JoinHostPort(hostname, strconv.Itoa(port))

	return Identity{
		StartTime:            startTime,
		Address:              address,
		Name:                 name,
		LastMessageTimestamp: 0,
		ID:                   strconv.FormatInt(startTime, 10) + "-" + name,
	}, nil
}

// ToProto is an adapter for converting between Identity and the protobuf
// Node equivalent.
func (i Identity) ToProto() *pb.Node {
	return &pb.Node{
		StartTime:              i.StartTime,
		Address:                i.Address,
		Name:                   i.Name,
		Id:                     i.ID,
		LatestMessageTimestamp: i.LastMessageTimestamp,
	}
}

// FromProto is an adapter for converting between a protobuf Node and the
// Identity equivalent.
func FromProto(node *pb.Node) Identity {
	return Identity{
		StartTime:            node.GetStartTime(),
		Address:              node.GetAddress(),
		Name:                 node.GetName(),
		ID:                   node.GetId(),
		LastMessageTimestamp: node.GetLatestMessageTimestamp(),
	}
}

func getPrivateIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
