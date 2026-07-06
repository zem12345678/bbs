package snowflake

import (
	"fmt"
	"sync"
	"time"
)

const (
	workerBits   = 10
	sequenceBits = 12
	workerMax    = -1 ^ (-1 << workerBits)
	sequenceMax  = -1 ^ (-1 << sequenceBits)
	timeShift    = workerBits + sequenceBits
	workerShift  = sequenceBits
)

// epoch: 2024-01-01 00:00:00 UTC
var epoch int64 = 1704067200000

type Node struct {
	mu        sync.Mutex
	timestamp int64
	workerID  int64
	sequence  int64
}

func NewNode(workerID int64) (*Node, error) {
	if workerID < 0 || workerID > workerMax {
		return nil, fmt.Errorf("worker ID must be between 0 and %d", workerMax)
	}
	return &Node{workerID: workerID}, nil
}

func (n *Node) Generate() int64 {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now().UnixMilli()
	if now == n.timestamp {
		n.sequence = (n.sequence + 1) & sequenceMax
		if n.sequence == 0 {
			for now <= n.timestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		n.sequence = 0
	}
	n.timestamp = now
	return (now-epoch)<<timeShift | n.workerID<<workerShift | n.sequence
}
