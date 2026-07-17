package credit

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInsufficientCredit         = errors.New("insufficient credit balance")
	ErrCreditLedgerMismatch       = errors.New("credit ledger does not match idempotency request")
	ErrInconsistentCreditTransfer = errors.New("inconsistent credit transfer ledger")
	ErrUnbalancedCreditTransfer   = errors.New("unbalanced credit transfer")
	ErrCreditReservationNotFound  = errors.New("credit reservation not found")
	ErrCreditReservationMismatch  = errors.New("credit reservation does not match settlement")
)

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

type CreditReservation struct {
	ID            int64
	UserID        int64
	Amount        int64
	Status        string
	Reason        string
	Description   string
	SourceEventID string
	SourceType    string
	SourceID      int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SettledAt     time.Time
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
	ReserveCredit(ctx context.Context, reservation CreditReservation, ledger LedgerEntry) (CreditReservation, Balance, bool, error)
	ReleaseCredit(ctx context.Context, reservation CreditReservation, ledger LedgerEntry) (CreditReservation, Balance, bool, error)
	SettleCreditReservation(ctx context.Context, reservation CreditReservation, credit LedgerEntry) error
	TransferCredit(ctx context.Context, debit LedgerEntry, credit LedgerEntry) error
	SavePendingArticleCredit(ctx context.Context, eventID, reason string, articleID, actorID, delta int64, sourceType string, sourceID int64, createdAt time.Time) error
	FlushPendingArticleCredits(ctx context.Context, article ArticleRef) error
	GetBalance(ctx context.Context, userID int64) (Balance, error)
	ListLedger(ctx context.Context, userID int64, limit, offset int32) ([]LedgerEntry, int64, Balance, error)
}
