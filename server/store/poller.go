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

/*type deleteReq struct {
	key       string
	timestamp int64
	performSync bool
	respCh    chan error
}*/

// StartPoller starts polling for operations to apply to the store in order to serialise access to the store's map.
// TODO: done channel for graceful shutdown
func (s *Store) StartPoller() {
	s.getReqChan = make(chan getReq, s.RequestChanBufSize)
	s.insertReqChan = make(chan insertReq, s.RequestChanBufSize)
	//s.deleteReqChan = make(chan deleteReq, s.RequestChanBufSize)
	//s.syncReqChan = make(chan deleteReq, s.RequestChanBufSize)

	go func() {
		for {
			select {
			case req := <-s.getReqChan:
				req.respCh <- s.performGetOperation(req.key)
			case req := <-s.insertReqChan:
				req.respCh <- s.performInsertOperation(req.key, req.value, req.timestamp, pb.OperationType_UPDATE, req.performSync)
				/*case req := <-s.deleteReqChan:
				req.respCh <- s.performInsertOperation(req.key, nil, req.timestamp, pb.OperationType_DELETE, req.performSync)*/
			}
		}
	}()
}
