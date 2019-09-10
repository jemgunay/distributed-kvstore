package store

import (
	"sync/atomic"

	pb "github.com/jemgunay/distributed-kvstore/proto"
	"golang.org/x/net/context"
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

type insertReq struct {
	key           string
	value         []byte
	timestamp     int64
	operationType pb.OperationType
	performSync   bool
	respCh        chan error
}

// StartPoller starts polling for operations to apply to the store in order to serialise access to the store's map.
func (s *Store) StartPoller() {
	s.getReqChan = make(chan *getReq, s.RequestChanBufSize)
	s.insertReqChan = make(chan *insertReq, s.RequestChanBufSize)
	s.syncRequestFeedChan = make(chan *pb.SyncMessage, s.SyncRequestFeedChanBufSize)
	s.unsubscribeChan = make(chan subscription, s.SyncRequestFeedChanBufSize)

	go func() {
		for {
			select {
			case req, ok := <-s.getReqChan:
				if ok {
					req.respCh <- s.performGetOperation(req.key)
				} else {
					s.getReqChan = nil
				}
			case req, ok := <-s.insertReqChan:
				if ok {
					result := s.performInsertOperation(req)
					req.respCh <- result

					// process subscriptions
					s.fanOutSubscriptions(req)
				} else {
					s.insertReqChan = nil
				}
			case sub := <-s.unsubscribeChan:
				close(sub.ch)
				sub.ch = nil
				// delete subscription from map
				subsByKey := s.subscriptions[sub.key]
				delete(subsByKey, sub.id)

				if len(subsByKey) > 0 {
					// reassign updated sub-map
					s.subscriptions[sub.key] = subsByKey
				} else {
					// clean up empty sub-maps, i.e. if the only element in that map was just deleted
					delete(s.subscriptions, sub.key)
				}
			}

			// break out of polling loop as both channels are drained and nil
			if s.getReqChan == nil && s.insertReqChan == nil {
				return
			}
		}
	}()
}

func (s *Store) fanOutSubscriptions(req *insertReq) {
	// determine if any subscriptions for that key exist
	if s.subscriptions[req.key] == nil {
		return
	}

	// push the request to each subscriber of that key
	for _, r := range s.subscriptions[req.key] {
		// kill subscription if insert request was a delete request
		if req.operationType == pb.OperationType_DELETE {
			r.cancel()
			continue
		}

		r.ch <- &pb.FetchResponse{
			Value:     req.value,
			Timestamp: req.timestamp,
		}
	}
}

// Subscribe hooks into the store to listen for insert requests from other nodes or clients. These are then forwarded
// on to the consumer via the response channel. The cancel func will unregister the subscription listener and gracefully
// clean up references in the store.
func (s *Store) Subscribe(ctx context.Context, key string) (chan *pb.FetchResponse, context.CancelFunc) {
	newID := atomic.AddUint64(&s.nextSubscriptionID, 1)

	// create the channel for providing updates to the subscriptions
	ctx, cancel := context.WithCancel(ctx)
	newSub := subscription{
		key:    key,
		id:     newID,
		ch:     make(chan *pb.FetchResponse, s.RequestChanBufSize),
		cancel: cancel,
	}

	if s.subscriptions[key] == nil {
		// create bucket for this key, and add subscription
		s.subscriptions[key] = map[uint64]subscription{
			newID: newSub,
		}
	} else {
		// add subscription into existing bucket
		s.subscriptions[key][newID] = newSub
	}

	// setup cancellation to provide the ability to unsubscribe
	go func() {
		<-ctx.Done()
		s.unsubscribeChan <- newSub
	}()

	return newSub.ch, cancel
}
