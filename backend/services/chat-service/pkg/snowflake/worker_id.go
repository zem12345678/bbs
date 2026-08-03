package snowflake

import (
	"fmt"
	"strconv"
	"strings"
)

// ResolveWorkerID keeps local fixed-worker configuration intact and derives a
// unique worker ID for a Kubernetes StatefulSet instance when instanceName is
// supplied. StatefulSet pod names end in a stable numeric ordinal.
func ResolveWorkerID(workerID, rangeStart, rangeSize int64, instanceName string) (int64, error) {
	instanceName = strings.TrimSpace(instanceName)
	if instanceName == "" {
		if workerID < 0 || workerID > maxWorkerID {
			return 0, fmt.Errorf("worker ID must be between 0 and %d", maxWorkerID)
		}
		return workerID, nil
	}
	if rangeStart < 0 || rangeStart > maxWorkerID {
		return 0, fmt.Errorf("worker ID range start must be between 0 and %d", maxWorkerID)
	}
	if rangeSize <= 0 || rangeSize > maxWorkerID+1-rangeStart {
		return 0, fmt.Errorf("worker ID range size must fit within 0 and %d", maxWorkerID)
	}
	ordinal, err := statefulSetOrdinal(instanceName)
	if err != nil {
		return 0, err
	}
	if ordinal >= rangeSize {
		return 0, fmt.Errorf("statefulset ordinal %d exceeds worker ID range size %d", ordinal, rangeSize)
	}
	return rangeStart + ordinal, nil
}

func statefulSetOrdinal(instanceName string) (int64, error) {
	separator := strings.LastIndex(instanceName, "-")
	if separator <= 0 || separator == len(instanceName)-1 {
		return 0, fmt.Errorf("instance name %q must end with a StatefulSet ordinal", instanceName)
	}
	ordinal, err := strconv.ParseInt(instanceName[separator+1:], 10, 64)
	if err != nil || ordinal < 0 {
		return 0, fmt.Errorf("instance name %q must end with a non-negative StatefulSet ordinal", instanceName)
	}
	return ordinal, nil
}
