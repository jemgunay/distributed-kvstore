package store

type getReq struct {
	key    string
	respCh chan getResp
}

type getResp struct {
	data      []byte
	timestamp int64
	err       error
}

type putReq struct {
	key    string
	value  []byte
	respCh chan error
}

type deleteReq struct {
	key    string
	respCh chan error
}

// StartPoller starts polling for operations to apply to the store in order to serialise access to the store's map.
// TODO: done channel for graceful shutdown
func (s *Store) StartPoller() {
	s.getReqChan = make(chan getReq, s.RequestChanBufSize)
	s.putReqChan = make(chan putReq, s.RequestChanBufSize)
	s.deleteReqChan = make(chan deleteReq, s.RequestChanBufSize)

	go func() {
		for {
			select {
			case req := <-s.getReqChan:
				req.respCh <- s.performGetOperation(req.key)
			case req := <-s.putReqChan:
				req.respCh <- s.performInsertOperation(req.key, req.value, updateOp)
			case req := <-s.deleteReqChan:
				req.respCh <- s.performInsertOperation(req.key, nil, deleteOp)
			}
		}
	}()
}
