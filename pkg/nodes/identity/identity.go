package identity

import (
	"net"
	"os"
	"strconv"
	"time"

	petname "github.com/dustinkirkland/golang-petname"

	pb "github.com/jemgunay/distributed-kvstore/pkg/proto"
)

func init() {
	petname.NonDeterministicMode()
}

type Identity struct {
	StartTime            int64
	Address              string
	Name                 string
	ID                   string
	LastMessageTimestamp int64
}

func New(port int) (Identity, error) {
	name := petname.Generate(3, "-")
	startTime := time.Now().UTC().UnixNano()

	hostname, err := os.Hostname()
	if err != nil {
		return Identity{}, err
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

func (i Identity) ToProto() *pb.Node {
	return &pb.Node{
		StartTime:              i.StartTime,
		Address:                i.Address,
		Name:                   i.Name,
		Id:                     i.ID,
		LatestMessageTimestamp: i.LastMessageTimestamp,
	}
}

func FromProto(node *pb.Node) Identity {
	return Identity{
		StartTime:            node.GetStartTime(),
		Address:              node.GetAddress(),
		Name:                 node.GetName(),
		ID:                   node.GetId(),
		LastMessageTimestamp: node.GetLatestMessageTimestamp(),
	}
}
