package admin

type SearchRebuildStatus struct {
	JobID          string `json:"job_id"`
	State          string `json:"state"`
	RequestedBy    int64  `json:"requested_by"`
	ArticleTotal   int64  `json:"article_total"`
	ArticleIndexed int64  `json:"article_indexed"`
	TopicTotal     int64  `json:"topic_total"`
	TopicIndexed   int64  `json:"topic_indexed"`
	StartedAt      int64  `json:"started_at"`
	CompletedAt    int64  `json:"completed_at"`
	UpdatedAt      int64  `json:"updated_at"`
	Error          string `json:"error"`
}
