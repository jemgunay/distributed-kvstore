package store

import (
	"fmt"

	pb "github.com/jemgunay/distributed-kvstore/pkg/proto"
)

type getReq struct {
	key    string
	respCh chan getResp
}

type getResp struct {
	data      []byte
	timestamp int64
	err       error
}

type modifyReq struct {
	key       string
	value     []byte
	timestamp int64
	operation pb.OperationVariant
	// performSync determines if the request should be replicated to other nodes.
	performSync bool
	respCh      chan error
}

// startPoller starts polling for latestOp to apply to the store in order to serialise access to the store's map.
func (s *Store) startPoller() {
	for {
		select {
		case req := <-s.getReqQueue:
			req.respCh <- s.performGetOperation(req.key)

		case req := <-s.modifyReqQueue:
			result := s.performModifyOperation(req)
			req.respCh <- result

			s.fanOutSubscriptions(req)

		case sub := <-s.subscribeQueue:
			s.registerSubscription(sub)
		}
	}
}

func (s *Store) unsubscribe(sub subscription) {
	// closing channel will cause the consuming connection to be aborted
	close(sub.stream)

	subsByKey := s.subscriptions[sub.key]
	if len(subsByKey) > 1 {
		// delete subscription from map & reassign updated sub-map
		delete(subsByKey, sub.id)
		s.subscriptions[sub.key] = subsByKey
	} else {
		// clean up empty sub-maps, i.e. if the only element in that map was just deleted
		delete(s.subscriptions, sub.key)
	}
}

func (s *Store) fanOutSubscriptions(req modifyReq) {
	// determine if any subscriptions for that key exist
	if s.subscriptions[req.key] == nil {
		return
	}

	resp := &pb.FetchResponse{
		Value:     req.value,
		Timestamp: req.timestamp,
	}

	// push the request to each subscriber of that key
	for _, sub := range s.subscriptions[req.key] {
		// TODO: document this feature:
		// kill subscription if insert request was a delete request
		if req.operation == pb.OperationVariant_DELETE {
			close(sub.stream)
			continue
		}

		select {
		case <-sub.ctx.Done():
			fmt.Println("cancelling subscription")
			s.unsubscribe(sub)
		case sub.stream <- resp:
		}
	}
}
