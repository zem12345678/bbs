package snowflake

import (
	"errors"
	"sync"
	"time"
)

const (
	epochMillis  int64 = 1704067200000
	workerBits         = 10
	sequenceBits       = 12
	maxWorkerID  int64 = 1<<workerBits - 1
	sequenceMask int64 = 1<<sequenceBits - 1
	workerShift        = sequenceBits
	timeShift          = workerBits + sequenceBits
)

var ErrClockMovedBackwards = errors.New("system clock moved backwards")

type Generator struct {
	mu       sync.Mutex
	workerID int64
	lastMS   int64
	sequence int64
	now      func() time.Time
}

func New(workerID int64) (*Generator, error) {
	if workerID < 0 || workerID > maxWorkerID {
		return nil, errors.New("snowflake worker id must be between 0 and 1023")
	}
	return &Generator{workerID: workerID, lastMS: -1, now: time.Now}, nil
}

func (g *Generator) Next() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	nowMS := g.now().UnixMilli()
	if nowMS < g.lastMS {
		return 0, ErrClockMovedBackwards
	}
	if nowMS == g.lastMS {
		g.sequence = (g.sequence + 1) & sequenceMask
		if g.sequence == 0 {
			for nowMS <= g.lastMS {
				time.Sleep(time.Millisecond)
				nowMS = g.now().UnixMilli()
			}
		}
	} else {
		g.sequence = 0
	}

	g.lastMS = nowMS
	elapsed := nowMS - epochMillis
	if elapsed < 0 {
		return 0, ErrClockMovedBackwards
	}
	return (elapsed << timeShift) | (g.workerID << workerShift) | g.sequence, nil
}
