package chat

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	domain "chat-service/internal/domain/chat"

	"github.com/google/uuid"
)

const (
	maxRoomNumberAttempts = 5
	roomNumberLength      = 8
	roomNumberAlphabet    = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	maxRoomNameRunes      = 80
	maxGroupNameRunes     = 40
	maxMessageRunes       = 4000
	maxAnnouncementRunes  = 4000
)

type IDGenerator interface {
	Next() (int64, error)
}

type Service struct {
	repo          domain.Repository
	ids           IDGenerator
	newRoomNumber func() (string, error)
	newEventID    func() string
}

func NewService(repo domain.Repository, ids IDGenerator) *Service {
	return &Service{
		repo:          repo,
		ids:           ids,
		newRoomNumber: randomRoomNumber,
		newEventID:    uuid.NewString,
	}
}

func (s *Service) CreateRoom(ctx context.Context, creatorID int64, name string) (domain.RoomDetails, error) {
	name, err := normalizedText(name, maxRoomNameRunes, "room name")
	if err != nil {
		return domain.RoomDetails{}, err
	}
	if creatorID <= 0 {
		return domain.RoomDetails{}, invalidInput("creator id is required")
	}

	roomID, err := s.ids.Next()
	if err != nil {
		return domain.RoomDetails{}, err
	}
	for attempt := 0; attempt < maxRoomNumberAttempts; attempt++ {
		roomNo, numberErr := s.newRoomNumber()
		if numberErr != nil {
			return domain.RoomDetails{}, numberErr
		}
		room := domain.Room{
			ID:        roomID,
			RoomNo:    roomNo,
			Name:      name,
			CreatorID: creatorID,
			Status:    domain.RoomStatusActive,
		}
		owner := domain.Membership{
			RoomID: roomID,
			UserID: creatorID,
			Role:   domain.MemberRoleOwner,
			Status: domain.MemberStatusJoined,
		}
		details, createErr := s.repo.CreateRoom(ctx, room, owner)
		if !errors.Is(createErr, domain.ErrRoomNumberConflict) {
			return details, createErr
		}
	}
	return domain.RoomDetails{}, domain.ErrRoomNumberConflict
}

func (s *Service) LookupRoom(ctx context.Context, roomNo string, userID int64) (domain.RoomDetails, error) {
	roomNo, err := normalizeRoomNumber(roomNo)
	if err != nil {
		return domain.RoomDetails{}, err
	}
	details, err := s.repo.LookupRoom(ctx, roomNo, userID)
	if err == nil && details.Membership == nil {
		details.Room.Announcement = ""
	}
	return details, err
}

func (s *Service) JoinRoom(ctx context.Context, roomNo string, userID int64) (domain.RoomDetails, error) {
	roomNo, err := normalizeRoomNumber(roomNo)
	if err != nil {
		return domain.RoomDetails{}, err
	}
	if userID <= 0 {
		return domain.RoomDetails{}, invalidInput("user id is required")
	}
	return s.repo.JoinRoom(ctx, roomNo, userID, s.newEventID())
}

func (s *Service) ListSidebar(ctx context.Context, userID int64) (domain.Sidebar, error) {
	if userID <= 0 {
		return domain.Sidebar{}, invalidInput("user id is required")
	}
	return s.repo.ListSidebar(ctx, userID)
}

func (s *Service) ListMessages(ctx context.Context, roomNo string, userID int64, query domain.MessageQuery) (domain.MessagePage, error) {
	roomNo, err := normalizeRoomNumber(roomNo)
	if err != nil {
		return domain.MessagePage{}, err
	}
	if userID <= 0 {
		return domain.MessagePage{}, invalidInput("user id is required")
	}
	if query.BeforeSeq > 0 && query.AfterSeq > 0 {
		return domain.MessagePage{}, invalidInput("before_seq and after_seq are mutually exclusive")
	}
	if (query.BeforeSeq > 0 || query.AfterSeq > 0) && (query.AnchorSeq > 0 || query.Before > 0 || query.After > 0) {
		return domain.MessagePage{}, invalidInput("directional and anchor pagination are mutually exclusive")
	}
	if query.BeforeSeq > 0 || query.AfterSeq > 0 {
		query.Limit = boundedLimit(query.Limit, 20, 100)
	} else {
		query.Before = boundedLimit(query.Before, 20, 100)
		query.After = boundedLimit(query.After, 20, 100)
	}
	return s.repo.ListMessages(ctx, roomNo, userID, query)
}

func (s *Service) SendMessage(ctx context.Context, roomNo string, userID int64, clientMessageID, body string) (domain.Message, int64, error) {
	roomNo, err := normalizeRoomNumber(roomNo)
	if err != nil {
		return domain.Message{}, 0, err
	}
	if userID <= 0 {
		return domain.Message{}, 0, invalidInput("user id is required")
	}
	parsedClientID, err := uuid.Parse(strings.TrimSpace(clientMessageID))
	if err != nil || parsedClientID == uuid.Nil {
		return domain.Message{}, 0, invalidInput("valid client_message_id is required")
	}
	body, err = normalizedText(body, maxMessageRunes, "message body")
	if err != nil {
		return domain.Message{}, 0, err
	}
	messageID, err := s.ids.Next()
	if err != nil {
		return domain.Message{}, 0, err
	}
	message := domain.Message{
		ID:              messageID,
		SenderID:        userID,
		ClientMessageID: parsedClientID.String(),
		Body:            body,
		Status:          domain.MessageStatusPublished,
	}
	return s.repo.SendMessage(ctx, roomNo, userID, message, s.newEventID())
}

func (s *Service) AdvanceRead(ctx context.Context, roomNo string, userID, readSeq int64) (domain.Membership, int64, error) {
	roomNo, err := normalizeRoomNumber(roomNo)
	if err != nil {
		return domain.Membership{}, 0, err
	}
	if userID <= 0 {
		return domain.Membership{}, 0, invalidInput("user id is required")
	}
	if readSeq < 0 {
		return domain.Membership{}, 0, invalidInput("read_seq cannot be negative")
	}
	return s.repo.AdvanceRead(ctx, roomNo, userID, readSeq, s.newEventID())
}

func (s *Service) CreateGroup(ctx context.Context, userID int64, name string, sortOrder int32) (domain.Group, error) {
	if userID <= 0 {
		return domain.Group{}, invalidInput("user id is required")
	}
	name, err := normalizedText(name, maxGroupNameRunes, "group name")
	if err != nil {
		return domain.Group{}, err
	}
	id, err := s.ids.Next()
	if err != nil {
		return domain.Group{}, err
	}
	return s.repo.CreateGroup(ctx, domain.Group{ID: id, UserID: userID, Name: name, SortOrder: sortOrder})
}

func (s *Service) UpdateGroup(ctx context.Context, userID, groupID int64, name string, sortOrder int32) (domain.Group, error) {
	if userID <= 0 || groupID <= 0 {
		return domain.Group{}, invalidInput("user id and group id are required")
	}
	name, err := normalizedText(name, maxGroupNameRunes, "group name")
	if err != nil {
		return domain.Group{}, err
	}
	return s.repo.UpdateGroup(ctx, domain.Group{ID: groupID, UserID: userID, Name: name, SortOrder: sortOrder})
}

func (s *Service) DeleteGroup(ctx context.Context, userID, groupID int64) error {
	if userID <= 0 || groupID <= 0 {
		return invalidInput("user id and group id are required")
	}
	return s.repo.DeleteGroup(ctx, userID, groupID)
}

func (s *Service) PlaceRoom(ctx context.Context, roomNo string, userID, groupID int64, sortOrder int32) (domain.Membership, error) {
	roomNo, err := normalizeRoomNumber(roomNo)
	if err != nil {
		return domain.Membership{}, err
	}
	if userID <= 0 || groupID < 0 {
		return domain.Membership{}, invalidInput("valid user id and group id are required")
	}
	return s.repo.PlaceRoom(ctx, roomNo, userID, domain.Placement{GroupID: groupID, SortOrder: sortOrder})
}

func (s *Service) UpdateAnnouncement(ctx context.Context, roomNo string, userID int64, announcement string) (domain.Room, error) {
	roomNo, err := normalizeRoomNumber(roomNo)
	if err != nil {
		return domain.Room{}, err
	}
	announcement = strings.TrimSpace(announcement)
	if utf8.RuneCountInString(announcement) > maxAnnouncementRunes {
		return domain.Room{}, invalidInput(fmt.Sprintf("announcement exceeds %d characters", maxAnnouncementRunes))
	}
	if userID <= 0 {
		return domain.Room{}, invalidInput("user id is required")
	}
	return s.repo.UpdateAnnouncement(ctx, roomNo, userID, announcement, s.newEventID())
}

func (s *Service) MarkAnnouncementSeen(ctx context.Context, roomNo string, userID, version int64) (domain.Membership, error) {
	roomNo, err := normalizeRoomNumber(roomNo)
	if err != nil {
		return domain.Membership{}, err
	}
	if userID <= 0 || version < 0 {
		return domain.Membership{}, invalidInput("valid user id and announcement version are required")
	}
	return s.repo.MarkAnnouncementSeen(ctx, roomNo, userID, version)
}

func (s *Service) ValidateMemberships(ctx context.Context, userID int64, roomNumbers []string) ([]string, error) {
	if userID <= 0 {
		return nil, invalidInput("user id is required")
	}
	unique := make([]string, 0, len(roomNumbers))
	seen := make(map[string]struct{}, len(roomNumbers))
	for _, value := range roomNumbers {
		roomNo, err := normalizeRoomNumber(value)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[roomNo]; exists {
			continue
		}
		seen[roomNo] = struct{}{}
		unique = append(unique, roomNo)
	}
	if len(unique) > 100 {
		return nil, invalidInput("at most 100 room subscriptions may be validated at once")
	}
	return s.repo.ValidateMemberships(ctx, userID, unique)
}

func normalizeRoomNumber(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != roomNumberLength {
		return "", invalidInput("room number must contain 8 characters")
	}
	for _, char := range value {
		if !strings.ContainsRune(roomNumberAlphabet, char) {
			return "", invalidInput("room number contains unsupported characters")
		}
	}
	return value, nil
}

func randomRoomNumber() (string, error) {
	buffer := make([]byte, roomNumberLength)
	random := make([]byte, roomNumberLength)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for index, value := range random {
		buffer[index] = roomNumberAlphabet[int(value)%len(roomNumberAlphabet)]
	}
	return string(buffer), nil
}

func normalizedText(value string, maxRunes int, label string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", invalidInput(fmt.Sprintf("%s is required", label))
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return "", invalidInput(fmt.Sprintf("%s exceeds %d characters", label, maxRunes))
	}
	return value, nil
}

func boundedLimit(value, fallback, maximum int32) int32 {
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

func invalidInput(message string) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidInput, message)
}
