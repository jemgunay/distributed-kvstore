# TODO

- Improve identification stage to retry/wait until all nodes are up (instead of waiting for X seconds before attempting).
- Improve error handling of failed syncs.
- Shard keys internally based on hash range, i.e. bucket=hash_num/(max_hash/num_shards)
- Subscribe stream operation to be consumed by clients to listen for changes to a data.
- Dashboard tool that polls each node to determine state/to visualise data availability.
- Instead of a node syncing with every other node, only sync with X number of nodes and allow them to cascade the request to other nodes. Use a bitmask to determine which nodes are aware of the new operation. Use another bitmask to determine which nodes are aware of distribution completion. Size of bitmask represents number of nodes - one bit per node, i.e. 0001000000000 -> 1111111111111 (completed)
- Don't complete publish request until it has successfully been stored to more than one node? Prevents data loss in the event of a node going down.
- If all nodes have updated record, then we can flatten the operations to just one operation (depending on operation types or time elapsed since last update) in order to reduce memory usage.
- Bringing a node out of the network and introducing new ones.