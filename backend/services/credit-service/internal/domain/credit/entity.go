package credit

import (
	"context"
	"errors"
	"time"
)

var ErrInsufficientCredit = errors.New("insufficient credit balance")

type Balance struct {
	UserID    int64
	Total     int64
	UpdatedAt time.Time
}

type LedgerEntry struct {
	ID            int64
	UserID        int64
	Delta         int64
	BalanceAfter  int64
	Reason        string
	Description   string
	SourceEventID string
	SourceType    string
	SourceID      int64
	CreatedAt     time.Time
}

type ArticleRef struct {
	ID       int64
	AuthorID int64
	Title    string
}

type Repository interface {
	EnsureSchema(ctx context.Context) error
	SaveArticle(ctx context.Context, article ArticleRef, publishedAt time.Time) error
	GetArticle(ctx context.Context, id int64) (ArticleRef, error)
	AddCredit(ctx context.Context, entry LedgerEntry) error
	AdjustCredit(ctx context.Context, entry LedgerEntry) (LedgerEntry, Balance, bool, error)
	DebitCredit(ctx context.Context, entry LedgerEntry) (LedgerEntry, Balance, bool, error)
	TransferCredit(ctx context.Context, debit LedgerEntry, credit LedgerEntry) error
	SavePendingArticleCredit(ctx context.Context, eventID, reason string, articleID, actorID, delta int64, sourceType string, sourceID int64, createdAt time.Time) error
	FlushPendingArticleCredits(ctx context.Context, article ArticleRef) error
	GetBalance(ctx context.Context, userID int64) (Balance, error)
	ListLedger(ctx context.Context, userID int64, limit, offset int32) ([]LedgerEntry, int64, Balance, error)
}
