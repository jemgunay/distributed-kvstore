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

var (
	// RequestChanBufSize is the size of each of the store poller request channel buffers.
	RequestChanBufSize = 1 << 10 // 1024

	getReqChan    chan getReq
	putReqChan    chan putReq
	deleteReqChan chan deleteReq
)

func StartPoller() {
	getReqChan = make(chan getReq, RequestChanBufSize)
	putReqChan = make(chan putReq, RequestChanBufSize)
	deleteReqChan = make(chan deleteReq, RequestChanBufSize)

	go func() {
		for {
			select {
			case req := <-getReqChan:
				req.respCh <- performGet(req.key)
			case req := <-putReqChan:
				req.respCh <- performInsertOperation(req.key, req.value, updateOp)
			case req := <-deleteReqChan:
				req.respCh <- performInsertOperation(req.key, nil, deleteOp)
			}
		}
	}()
}
