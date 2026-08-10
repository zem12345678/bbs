package admin

type Channel struct {
	ID             int64
	OwnerID        int64
	CategoryID     int64
	Name           string
	Description    string
	Color          string
	IsArchived     bool
	FollowersCount int64
	TopicsCount    int64
	LastPostedAt   int64
	CreatedAt      int64
	UpdatedAt      int64
	IsFeatured     bool
}

type ChannelList struct {
	Items []Channel
	Total int64
}
