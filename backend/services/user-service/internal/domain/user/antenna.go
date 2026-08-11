package user

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxAntennasPerOwner = 20
	MaxAntennaNameRunes = 100
)

var antennaSources = map[string]struct{}{
	"home": {}, "all": {}, "users": {}, "list": {}, "users_blacklist": {},
}

type Antenna struct {
	ID                             int64
	OwnerID                        int64
	Name                           string
	Source                         string
	UserListID                     int64
	Keywords                       [][]string
	ExcludeKeywords                [][]string
	Users                          []string
	CaseSensitive                  bool
	LocalOnly                      bool
	ExcludeBots                    bool
	WithReplies                    bool
	WithFile                       bool
	ExcludeNotesInSensitiveChannel bool
	IsActive                       bool
	CreatedAt                      time.Time
	UpdatedAt                      time.Time
	LastUsedAt                     time.Time
}

func NewAntenna(id, ownerID int64, name, source string, userListID int64, keywords, excludeKeywords [][]string, users []string, caseSensitive, localOnly, excludeBots, withReplies, withFile, excludeSensitive bool) (*Antenna, error) {
	now := time.Now()
	antenna := &Antenna{
		ID: id, OwnerID: ownerID, Name: name, Source: source, UserListID: userListID,
		Keywords: keywords, ExcludeKeywords: excludeKeywords, Users: users,
		CaseSensitive: caseSensitive, LocalOnly: localOnly, ExcludeBots: excludeBots,
		WithReplies: withReplies, WithFile: withFile,
		ExcludeNotesInSensitiveChannel: excludeSensitive,
		IsActive:                       true, CreatedAt: now, UpdatedAt: now, LastUsedAt: now,
	}
	if err := antenna.Validate(); err != nil {
		return nil, err
	}
	return antenna, nil
}

func (a *Antenna) Validate() error {
	if a == nil || a.ID <= 0 || a.OwnerID <= 0 {
		return ErrInvalidID
	}
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		return ErrAntennaNameRequired
	}
	if utf8.RuneCountInString(a.Name) > MaxAntennaNameRunes {
		return ErrAntennaNameTooLong
	}
	a.Source = strings.ToLower(strings.TrimSpace(a.Source))
	if _, ok := antennaSources[a.Source]; !ok {
		return ErrAntennaSourceInvalid
	}
	if a.Source == "list" && a.UserListID <= 0 {
		return ErrAntennaUserListRequired
	}
	if a.Source != "list" {
		a.UserListID = 0
	}
	a.Keywords = normalizeAntennaKeywords(a.Keywords)
	a.ExcludeKeywords = normalizeAntennaKeywords(a.ExcludeKeywords)
	if len(a.Keywords) == 0 && len(a.ExcludeKeywords) == 0 {
		return ErrAntennaKeywordsRequired
	}
	a.Users = normalizeAntennaUsers(a.Users)
	if a.Source == "users" && len(a.Users) == 0 {
		return ErrAntennaUsersRequired
	}
	return nil
}

func normalizeAntennaKeywords(groups [][]string) [][]string {
	out := make([][]string, 0, len(groups))
	for _, group := range groups {
		terms := make([]string, 0, len(group))
		for _, term := range group {
			if term = strings.TrimSpace(term); term != "" {
				terms = append(terms, term)
			}
		}
		if len(terms) > 0 {
			out = append(out, terms)
		}
	}
	return out
}

func normalizeAntennaUsers(users []string) []string {
	seen := make(map[string]struct{}, len(users))
	out := make([]string, 0, len(users))
	for _, user := range users {
		user = strings.TrimPrefix(strings.TrimSpace(user), "@")
		user = strings.TrimSpace(user)
		if user == "" {
			continue
		}
		key := strings.ToLower(user)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, user)
	}
	return out
}
