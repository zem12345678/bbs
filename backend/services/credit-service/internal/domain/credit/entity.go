package credit

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInsufficientCredit            = errors.New("insufficient credit balance")
	ErrCreditLedgerMismatch          = errors.New("credit ledger does not match idempotency request")
	ErrInconsistentCreditTransfer    = errors.New("inconsistent credit transfer ledger")
	ErrUnbalancedCreditTransfer      = errors.New("unbalanced credit transfer")
	ErrInvalidCreditTransfer         = errors.New("invalid credit transfer")
	ErrCreditReservationNotFound     = errors.New("credit reservation not found")
	ErrCreditReservationMismatch     = errors.New("credit reservation does not match settlement")
	ErrQAAcceptanceSettlementPending = errors.New("qa acceptance settlement pending")
	ErrCheckInStateMismatch          = errors.New("check-in state does not match credit ledger")
	ErrCheckInDayRegression          = errors.New("check-in day is before latest record")
	ErrInvalidTaskClaim              = errors.New("invalid task claim")
	ErrUnsupportedTask               = errors.New("unsupported task")
	ErrTaskNotCompleted              = errors.New("task completion requirement not met")
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

type QAAcceptanceReversal struct {
	QuestionAuthorID        int64
	TopicID                 int64
	AcceptedCommentID       int64
	AcceptedCommentAuthorID int64
	Amount                  int64
	AcceptedEventID         string
	ReversalEventID         string
	AnswererDescription     string
	QuestionerDescription   string
	OccurredAt              time.Time
}

type CheckIn struct {
	ID              int64
	UserID          int64
	LatestDay       string
	ConsecutiveDays int32
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type TaskClaimStatus struct {
	TaskID    int64
	TaskKey   string
	Cycle     string
	Completed bool
	Claimed   bool
}

type ArticleRef struct {
	ID       int64
	AuthorID int64
	Title    string
}

type TopicPublicationRef struct {
	ID       int64
	AuthorID int64
	Title    string
}

type CommentCreationRef struct {
	ID         int64
	AuthorID   int64
	EntityType string
	EntityID   int64
}

type Repository interface {
	EnsureSchema(ctx context.Context) error
	SaveArticle(ctx context.Context, article ArticleRef, publishedAt time.Time) error
	GetArticle(ctx context.Context, id int64) (ArticleRef, error)
	SavePublishedTopic(ctx context.Context, topic TopicPublicationRef, publishedAt time.Time) error
	HasPublishedTopic(ctx context.Context, userID int64) (bool, error)
	SaveCreatedComment(ctx context.Context, comment CommentCreationRef, createdAt time.Time) error
	HasCreatedComment(ctx context.Context, userID int64) (bool, error)
	AddCredit(ctx context.Context, entry LedgerEntry) error
	AdjustCredit(ctx context.Context, entry LedgerEntry) (LedgerEntry, Balance, bool, error)
	DebitCredit(ctx context.Context, entry LedgerEntry) (LedgerEntry, Balance, bool, error)
	ReserveCredit(ctx context.Context, reservation CreditReservation, ledger LedgerEntry) (CreditReservation, Balance, bool, error)
	ReleaseCredit(ctx context.Context, reservation CreditReservation, ledger LedgerEntry) (CreditReservation, Balance, bool, error)
	GetCheckIn(ctx context.Context, userID int64) (CheckIn, error)
	RecordCheckIn(ctx context.Context, checkIn CheckIn, ledger LedgerEntry) (CheckIn, LedgerEntry, Balance, bool, error)
	SettleCreditReservation(ctx context.Context, reservation CreditReservation, credit LedgerEntry) error
	ReverseQAAcceptance(ctx context.Context, reversal QAAcceptanceReversal) (bool, error)
	TransferCredit(ctx context.Context, debit LedgerEntry, credit LedgerEntry) error
	SavePendingArticleCredit(ctx context.Context, eventID, reason string, articleID, actorID, delta int64, sourceType string, sourceID int64, createdAt time.Time) error
	FlushPendingArticleCredits(ctx context.Context, article ArticleRef) error
	GetBalance(ctx context.Context, userID int64) (Balance, error)
	ListLedger(ctx context.Context, userID int64, limit, offset int32) ([]LedgerEntry, int64, Balance, error)
	GetLedgerEntry(ctx context.Context, userID int64, sourceEventID, reason string) (LedgerEntry, bool, error)
}
