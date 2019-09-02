package store

import (
	pb "github.com/jemgunay/distributed-kvstore/proto"
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
					req.respCh <- s.performInsertOperation(req)
				} else {
					s.insertReqChan = nil
				}
			}

			// break out of polling loop is both channels are drained and nil
			if s.getReqChan == nil && s.insertReqChan == nil {
				return
			}
		}
	}()
}
