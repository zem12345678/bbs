package article

import "context"

type ActiveUsersChartBucket struct {
	WriterUserIDs []int64
}

type ActiveUsersChart struct {
	Buckets []ActiveUsersChartBucket
}

type ActiveUsersChartRepository interface {
	GetActiveUsersChart(context.Context, NoteChartQuery) (ActiveUsersChart, error)
}
