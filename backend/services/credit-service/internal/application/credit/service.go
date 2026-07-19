package credit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "credit-service/internal/domain/credit"
)

const (
	WelcomeDelta          int64 = 20
	ArticlePublishedDelta int64 = 10
	CommentCreatedDelta   int64 = 3
	FollowDelta           int64 = 1
	LikeGivenDelta        int64 = 1
	FavoriteGivenDelta    int64 = 2
	CommentReceivedDelta  int64 = 1
	LikeReceivedDelta     int64 = 1
	FavoriteReceivedDelta int64 = 2
	QAAcceptedDelta       int64 = 10
	DailyCheckInDelta     int64 = 5

	CreditReservationStatusActive   = "ACTIVE"
	CreditReservationStatusReleased = "RELEASED"
	CreditReservationStatusSettled  = "SETTLED"
	QABountyReservationReason       = "qa_bounty_reserved"
	QABountyReleaseReason           = "qa_bounty_released"
	QAAnswerAcceptedReason          = "qa_answer_accepted"
	QAAnswerUnacceptedReason        = "qa_answer_unaccepted"
	QABountyRefundReason            = "qa_bounty_refunded"
)

var checkInLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

type Service struct {
	repo domain.Repository
}

type TransferCreditsCommand struct {
	PayerUserID       int64
	PayeeUserID       int64
	Amount            int64
	DebitReason       string
	DebitDescription  string
	CreditReason      string
	CreditDescription string
	SourceEventID     string
	SourceType        string
	SourceID          int64
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetBalance(ctx context.Context, userID int64) (domain.Balance, error) {
	return s.repo.GetBalance(ctx, userID)
}

func (s *Service) ListLedger(ctx context.Context, userID int64, limit, offset int32) ([]domain.LedgerEntry, int64, domain.Balance, error) {
	return s.repo.ListLedger(ctx, userID, limit, offset)
}

func (s *Service) GetCheckInStatus(ctx context.Context, userID int64, occurredAt time.Time) (domain.CheckIn, bool, error) {
	if userID <= 0 {
		return domain.CheckIn{}, false, errors.New("user id is required")
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	checkIn, err := s.repo.GetCheckIn(ctx, userID)
	if err != nil {
		return domain.CheckIn{}, false, err
	}
	return checkIn, checkIn.LatestDay == dailyCheckInDay(occurredAt), nil
}

func (s *Service) DailyCheckIn(ctx context.Context, userID int64, occurredAt time.Time) (domain.CheckIn, domain.LedgerEntry, domain.Balance, bool, error) {
	if userID <= 0 {
		return domain.CheckIn{}, domain.LedgerEntry{}, domain.Balance{}, false, errors.New("user id is required")
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	day := dailyCheckInDay(occurredAt)
	return s.repo.RecordCheckIn(ctx, domain.CheckIn{
		UserID:    userID,
		LatestDay: day,
	}, domain.LedgerEntry{
		UserID:        userID,
		Delta:         DailyCheckInDelta,
		Reason:        "daily_check_in",
		Description:   "每日签到奖励",
		SourceEventID: DailyCheckInEventID(userID, day),
		SourceType:    "check_in",
		CreatedAt:     occurredAt,
	})
}

func DailyCheckInEventID(userID int64, day string) string {
	return fmt.Sprintf("credit.checkin:%d:%s", userID, day)
}

func dailyCheckInDay(occurredAt time.Time) string {
	return occurredAt.In(checkInLocation).Format("2006-01-02")
}

func (s *Service) DebitCredits(ctx context.Context, userID, amount int64, reason, description, sourceEventID, sourceType string, sourceID int64, occurredAt time.Time) (domain.LedgerEntry, domain.Balance, bool, error) {
	if userID <= 0 {
		return domain.LedgerEntry{}, domain.Balance{}, false, errors.New("user id is required")
	}
	if amount <= 0 {
		return domain.LedgerEntry{}, domain.Balance{}, false, errors.New("debit amount must be positive")
	}
	sourceEventID = strings.TrimSpace(sourceEventID)
	if sourceEventID == "" {
		return domain.LedgerEntry{}, domain.Balance{}, false, errors.New("source event id is required")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "credit_debit"
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	return s.repo.DebitCredit(ctx, domain.LedgerEntry{
		UserID:        userID,
		Delta:         -amount,
		Reason:        reason,
		Description:   strings.TrimSpace(description),
		SourceEventID: sourceEventID,
		SourceType:    strings.TrimSpace(sourceType),
		SourceID:      sourceID,
		CreatedAt:     occurredAt,
	})
}

func (s *Service) AdjustCredits(ctx context.Context, userID, delta int64, reason, description, sourceEventID, sourceType string, sourceID int64, occurredAt time.Time) (domain.LedgerEntry, domain.Balance, bool, error) {
	if userID <= 0 {
		return domain.LedgerEntry{}, domain.Balance{}, false, errors.New("user id is required")
	}
	if delta == 0 {
		return domain.LedgerEntry{}, domain.Balance{}, false, errors.New("credit delta must not be zero")
	}
	sourceEventID = strings.TrimSpace(sourceEventID)
	if sourceEventID == "" {
		return domain.LedgerEntry{}, domain.Balance{}, false, errors.New("source event id is required")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "admin_adjustment"
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	return s.repo.AdjustCredit(ctx, domain.LedgerEntry{
		UserID:        userID,
		Delta:         delta,
		Reason:        reason,
		Description:   strings.TrimSpace(description),
		SourceEventID: sourceEventID,
		SourceType:    strings.TrimSpace(sourceType),
		SourceID:      sourceID,
		CreatedAt:     occurredAt,
	})
}

func (s *Service) TransferCredits(ctx context.Context, command TransferCreditsCommand) error {
	command.DebitReason = strings.TrimSpace(command.DebitReason)
	command.DebitDescription = strings.TrimSpace(command.DebitDescription)
	command.CreditReason = strings.TrimSpace(command.CreditReason)
	command.CreditDescription = strings.TrimSpace(command.CreditDescription)
	command.SourceEventID = strings.TrimSpace(command.SourceEventID)
	command.SourceType = strings.TrimSpace(command.SourceType)
	if command.PayerUserID <= 0 || command.PayeeUserID <= 0 || command.PayerUserID == command.PayeeUserID || command.Amount <= 0 || command.DebitReason == "" || command.CreditReason == "" || command.SourceEventID == "" {
		return domain.ErrInvalidCreditTransfer
	}
	occurredAt := time.Now()
	return s.repo.TransferCredit(ctx, domain.LedgerEntry{
		UserID:        command.PayerUserID,
		Delta:         -command.Amount,
		Reason:        command.DebitReason,
		Description:   command.DebitDescription,
		SourceEventID: command.SourceEventID,
		SourceType:    command.SourceType,
		SourceID:      command.SourceID,
		CreatedAt:     occurredAt,
	}, domain.LedgerEntry{
		UserID:        command.PayeeUserID,
		Delta:         command.Amount,
		Reason:        command.CreditReason,
		Description:   command.CreditDescription,
		SourceEventID: command.SourceEventID,
		SourceType:    command.SourceType,
		SourceID:      command.SourceID,
		CreatedAt:     occurredAt,
	})
}

func (s *Service) ReserveCredits(ctx context.Context, userID, amount int64, reason, description, sourceEventID, sourceType string, sourceID int64, occurredAt time.Time) (domain.CreditReservation, domain.Balance, bool, error) {
	if userID <= 0 {
		return domain.CreditReservation{}, domain.Balance{}, false, errors.New("user id is required")
	}
	if amount <= 0 {
		return domain.CreditReservation{}, domain.Balance{}, false, errors.New("reservation amount must be positive")
	}
	sourceEventID = strings.TrimSpace(sourceEventID)
	if sourceEventID == "" {
		return domain.CreditReservation{}, domain.Balance{}, false, errors.New("source event id is required")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "credit_reserved"
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	description = strings.TrimSpace(description)
	sourceType = strings.TrimSpace(sourceType)
	reservation := domain.CreditReservation{
		UserID:        userID,
		Amount:        amount,
		Status:        CreditReservationStatusActive,
		Reason:        reason,
		Description:   description,
		SourceEventID: sourceEventID,
		SourceType:    sourceType,
		SourceID:      sourceID,
		CreatedAt:     occurredAt,
	}
	ledger := domain.LedgerEntry{
		UserID:        userID,
		Delta:         -amount,
		Reason:        reason,
		Description:   description,
		SourceEventID: sourceEventID,
		SourceType:    sourceType,
		SourceID:      sourceID,
		CreatedAt:     occurredAt,
	}
	return s.repo.ReserveCredit(ctx, reservation, ledger)
}

func (s *Service) ReleaseCredits(ctx context.Context, userID, amount int64, reservationReason, releaseReason, description, sourceEventID, sourceType string, sourceID int64, occurredAt time.Time) (domain.CreditReservation, domain.Balance, bool, error) {
	if userID <= 0 {
		return domain.CreditReservation{}, domain.Balance{}, false, errors.New("user id is required")
	}
	if amount <= 0 {
		return domain.CreditReservation{}, domain.Balance{}, false, errors.New("release amount must be positive")
	}
	sourceEventID = strings.TrimSpace(sourceEventID)
	if sourceEventID == "" {
		return domain.CreditReservation{}, domain.Balance{}, false, errors.New("source event id is required")
	}
	reservationReason = strings.TrimSpace(reservationReason)
	if reservationReason == "" {
		reservationReason = "credit_reserved"
	}
	releaseReason = strings.TrimSpace(releaseReason)
	if releaseReason == "" {
		releaseReason = "credit_released"
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	description = strings.TrimSpace(description)
	sourceType = strings.TrimSpace(sourceType)
	reservation := domain.CreditReservation{
		UserID:        userID,
		Amount:        amount,
		Reason:        reservationReason,
		SourceEventID: sourceEventID,
		SourceType:    sourceType,
		SourceID:      sourceID,
	}
	ledger := domain.LedgerEntry{
		UserID:        userID,
		Delta:         amount,
		Reason:        releaseReason,
		Description:   description,
		SourceEventID: sourceEventID,
		SourceType:    sourceType,
		SourceID:      sourceID,
		CreatedAt:     occurredAt,
	}
	return s.repo.ReleaseCredit(ctx, reservation, ledger)
}

func (s *Service) HandleUserCreated(ctx context.Context, eventID string, userID int64, occurredAt time.Time) error {
	return s.add(ctx, eventID, userID, WelcomeDelta, "welcome", "注册奖励", "user", userID, occurredAt)
}

func (s *Service) HandleFollowed(ctx context.Context, eventID string, followerID, followeeID int64, occurredAt time.Time) error {
	if followerID <= 0 || followeeID <= 0 || followerID == followeeID {
		return nil
	}
	return s.add(ctx, eventID, followerID, FollowDelta, "follow_user", fmt.Sprintf("关注用户 #%d", followeeID), "user", followeeID, occurredAt)
}

func (s *Service) HandleArticlePublished(ctx context.Context, eventID string, articleID, authorID int64, title string, occurredAt time.Time) error {
	article := domain.ArticleRef{ID: articleID, AuthorID: authorID, Title: title}
	if err := s.repo.SaveArticle(ctx, article, occurredAt); err != nil {
		return err
	}
	if err := s.add(ctx, eventID, authorID, ArticlePublishedDelta, "article_published", fmt.Sprintf("发布文章《%s》", title), "article", articleID, occurredAt); err != nil {
		return err
	}
	return s.repo.FlushPendingArticleCredits(ctx, article)
}

func (s *Service) HandleCommentCreated(ctx context.Context, eventID string, commentID, articleID, authorID int64, occurredAt time.Time) error {
	if err := s.add(ctx, eventID, authorID, CommentCreatedDelta, "comment_created", fmt.Sprintf("评论文章 #%d", articleID), "comment", commentID, occurredAt); err != nil {
		return err
	}
	return s.addArticleOwnerCredit(ctx, eventID, "article_commented", articleID, authorID, CommentReceivedDelta, "comment", commentID, occurredAt)
}

func (s *Service) HandleReaction(ctx context.Context, eventID, eventType string, articleID, actorID int64, changed bool, occurredAt time.Time) error {
	if !changed {
		return nil
	}
	switch eventType {
	case "reaction.liked.v1":
		if err := s.add(ctx, eventID, actorID, LikeGivenDelta, "like_given", fmt.Sprintf("点赞文章 #%d", articleID), "article", articleID, occurredAt); err != nil {
			return err
		}
		return s.addArticleOwnerCredit(ctx, eventID, "article_liked", articleID, actorID, LikeReceivedDelta, "article", articleID, occurredAt)
	case "reaction.favorited.v1":
		if err := s.add(ctx, eventID, actorID, FavoriteGivenDelta, "favorite_given", fmt.Sprintf("收藏文章 #%d", articleID), "article", articleID, occurredAt); err != nil {
			return err
		}
		return s.addArticleOwnerCredit(ctx, eventID, "article_favorited", articleID, actorID, FavoriteReceivedDelta, "article", articleID, occurredAt)
	default:
		return nil
	}
}

func (s *Service) HandleQAAccepted(ctx context.Context, eventID string, topicID int64, title string, questionAuthorID, acceptedCommentID, acceptedCommentAuthorID, rewardCredits int64, occurredAt time.Time) error {
	return s.HandleQAAcceptedWithCycle(ctx, eventID, topicID, title, questionAuthorID, acceptedCommentID, acceptedCommentAuthorID, rewardCredits, 0, occurredAt)
}

func (s *Service) HandleQAAcceptedWithCycle(ctx context.Context, eventID string, topicID int64, title string, questionAuthorID, acceptedCommentID, acceptedCommentAuthorID, rewardCredits, acceptanceCycle int64, occurredAt time.Time) error {
	if questionAuthorID <= 0 || acceptedCommentAuthorID <= 0 || acceptedCommentID <= 0 || topicID <= 0 {
		return nil
	}
	if questionAuthorID == acceptedCommentAuthorID {
		return nil
	}
	eventID = QAAcceptedEventIDForCycle(topicID, acceptedCommentID, acceptanceCycle)
	if rewardCredits <= 0 {
		rewardCredits = QAAcceptedDelta
	}
	topicTitle := strings.TrimSpace(title)
	description := fmt.Sprintf("采纳答案悬赏：话题《%s》", topicTitle)
	if topicTitle == "" {
		description = fmt.Sprintf("采纳答案悬赏：话题 #%d", topicID)
	}
	rewardDescription := fmt.Sprintf("你的回答被采纳：话题《%s》", topicTitle)
	if topicTitle == "" {
		rewardDescription = fmt.Sprintf("你的回答被采纳：话题 #%d", topicID)
	}
	reservation := domain.CreditReservation{
		UserID:        questionAuthorID,
		Amount:        rewardCredits,
		Reason:        QABountyReservationReason,
		SourceEventID: QABountyReservationEventID(topicID),
		SourceType:    "topic",
		SourceID:      topicID,
	}
	reward := domain.LedgerEntry{
		UserID:        acceptedCommentAuthorID,
		Delta:         rewardCredits,
		Reason:        QAAnswerAcceptedReason,
		Description:   rewardDescription,
		SourceEventID: eventID,
		SourceType:    "comment",
		SourceID:      acceptedCommentID,
		CreatedAt:     occurredAt,
	}
	if err := s.repo.SettleCreditReservation(ctx, reservation, reward); err == nil {
		return nil
	} else if !errors.Is(err, domain.ErrCreditReservationNotFound) {
		return err
	}
	return s.repo.TransferCredit(ctx, domain.LedgerEntry{
		UserID:        questionAuthorID,
		Delta:         -rewardCredits,
		Reason:        "qa_bounty_paid",
		Description:   description,
		SourceEventID: eventID,
		SourceType:    "topic",
		SourceID:      topicID,
		CreatedAt:     occurredAt,
	}, reward)
}

func (s *Service) ReverseQAAcceptance(ctx context.Context, questionAuthorID, topicID, acceptedCommentID, acceptedCommentAuthorID, rewardCredits, acceptanceCycle int64, title string, occurredAt time.Time) (bool, error) {
	if questionAuthorID <= 0 || topicID <= 0 || acceptedCommentID <= 0 || acceptedCommentAuthorID <= 0 || questionAuthorID == acceptedCommentAuthorID {
		return false, errors.New("invalid qa acceptance reversal")
	}
	if rewardCredits <= 0 {
		rewardCredits = QAAcceptedDelta
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	topicTitle := strings.TrimSpace(title)
	answererDescription := fmt.Sprintf("撤销采纳后收回悬赏：话题《%s》", topicTitle)
	questionerDescription := fmt.Sprintf("撤销采纳返还悬赏：话题《%s》", topicTitle)
	if topicTitle == "" {
		answererDescription = fmt.Sprintf("撤销采纳后收回悬赏：话题 #%d", topicID)
		questionerDescription = fmt.Sprintf("撤销采纳返还悬赏：话题 #%d", topicID)
	}
	return s.repo.ReverseQAAcceptance(ctx, domain.QAAcceptanceReversal{
		QuestionAuthorID:        questionAuthorID,
		TopicID:                 topicID,
		AcceptedCommentID:       acceptedCommentID,
		AcceptedCommentAuthorID: acceptedCommentAuthorID,
		Amount:                  rewardCredits,
		AcceptedEventID:         QAAcceptedEventIDForCycle(topicID, acceptedCommentID, acceptanceCycle),
		ReversalEventID:         QAAcceptanceReversalEventID(topicID, acceptedCommentID, acceptanceCycle),
		AnswererDescription:     answererDescription,
		QuestionerDescription:   questionerDescription,
		OccurredAt:              occurredAt,
	})
}

func QABountyReservationEventID(topicID int64) string {
	return fmt.Sprintf("content.qa.bounty:%d", topicID)
}

func QAAcceptedEventID(topicID, commentID int64) string {
	return fmt.Sprintf("content.qa.accepted:%d:%d", topicID, commentID)
}

func QAAcceptedEventIDForCycle(topicID, commentID, cycle int64) string {
	if cycle <= 0 {
		return QAAcceptedEventID(topicID, commentID)
	}
	return fmt.Sprintf("content.qa.accepted:%d:%d:%d", topicID, commentID, cycle)
}

func QAAcceptanceReversalEventID(topicID, commentID, cycle int64) string {
	if cycle <= 0 {
		return fmt.Sprintf("content.qa.unaccepted:%d:%d", topicID, commentID)
	}
	return fmt.Sprintf("content.qa.unaccepted:%d:%d:%d", topicID, commentID, cycle)
}

func (s *Service) addArticleOwnerCredit(ctx context.Context, eventID, reason string, articleID, actorID, delta int64, sourceType string, sourceID int64, occurredAt time.Time) error {
	article, err := s.repo.GetArticle(ctx, articleID)
	if err != nil {
		return err
	}
	if article.AuthorID <= 0 {
		return s.repo.SavePendingArticleCredit(ctx, eventID, reason, articleID, actorID, delta, sourceType, sourceID, occurredAt)
	}
	if article.AuthorID == actorID {
		return nil
	}
	return s.add(ctx, eventID, article.AuthorID, delta, reason, ownerDescription(reason, actorID, article.Title), sourceType, sourceID, occurredAt)
}

func (s *Service) add(ctx context.Context, eventID string, userID, delta int64, reason, description, sourceType string, sourceID int64, occurredAt time.Time) error {
	return s.repo.AddCredit(ctx, domain.LedgerEntry{
		UserID:        userID,
		Delta:         delta,
		Reason:        reason,
		Description:   description,
		SourceEventID: eventID,
		SourceType:    sourceType,
		SourceID:      sourceID,
		CreatedAt:     occurredAt,
	})
}

func ownerDescription(reason string, actorID int64, title string) string {
	switch reason {
	case "article_commented":
		return fmt.Sprintf("用户 #%d 评论了你的文章《%s》", actorID, title)
	case "article_liked":
		return fmt.Sprintf("用户 #%d 点赞了你的文章《%s》", actorID, title)
	case "article_favorited":
		return fmt.Sprintf("用户 #%d 收藏了你的文章《%s》", actorID, title)
	default:
		return fmt.Sprintf("文章《%s》收到互动", title)
	}
}
