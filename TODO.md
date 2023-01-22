# TODO

- Improve error handling of failed syncs.
- More tests
  - Hefty payloads
- Store Shutdown().
- Shard keys internally based on hash range, i.e. bucket=hash_num/(max_hash/num_shards)
- Dashboard tool that polls each node to determine state/to visualise data availability.
- Instead of a node syncing with every other node, only sync with X number of nodes and allow them to cascade the request to other nodes. Use a bitmask to determine which nodes are aware of the new operation. Use another bitmask to determine which nodes are aware of distribution completion. Size of bitmask represents number of nodes - one bit per node, i.e. 0001000000000 -> 1111111111111 (completed)
- Don't complete publish request until it has successfully been stored to more than one node? Prevents data loss in the event of a node going down.
- If all nodes have updated record, then we can flatten the operations to just one operation (depending on operation types or time elapsed since last update) in order to reduce memory usage.

## Revisit

- Refactor dir structure
- Dockerise startup
- Zap logger
- Bubbletea/cobra for tasty CLI?
- Require ctx for API methods
- Bringing a node out of the network and introducing new ones.
  - Auto discovery, given port range?
- Configurable store/server/client with opts
- Require pkg for tests
- Implementation details section in readme
- Use time.Time instead of timestamp at API-level - epoch for internal comms
- Status endpoints, returns health/queue backpressure + queue capacity  
  - Monitor service to probe status of nodes. Visualise which nodes/keys are out of sync
  - Subscriber backpressure
- Prevent subscription to a key that does not exist? Aligns with delete triggering an unsub 
- Document 
  - key must be defined
  - blob-ing
    - gzip big payloads? optional config?
  - timestamp dependent, clock drift will break stuff