package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"sort"
	"strconv"
	"strings"

	"api-gateway/api/proto/feedpb"
	"api-gateway/api/proto/userpb"
	"api-gateway/pkg/http/response"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type antennaJSON struct {
	AntennaID                      string      `json:"antennaId"`
	Name                           *string     `json:"name"`
	Source                         *string     `json:"src"`
	UserListID                     *string     `json:"userListId"`
	Keywords                       *[][]string `json:"keywords"`
	ExcludeKeywords                *[][]string `json:"excludeKeywords"`
	Users                          *[]string   `json:"users"`
	CaseSensitive                  *bool       `json:"caseSensitive"`
	LocalOnly                      *bool       `json:"localOnly"`
	ExcludeBots                    *bool       `json:"excludeBots"`
	WithReplies                    *bool       `json:"withReplies"`
	WithFile                       *bool       `json:"withFile"`
	ExcludeNotesInSensitiveChannel *bool       `json:"excludeNotesInSensitiveChannel"`
	Limit                          int32       `json:"limit"`
	SinceID                        string      `json:"sinceId"`
	UntilID                        string      `json:"untilId"`
	SinceDate                      int64       `json:"sinceDate"`
	UntilDate                      int64       `json:"untilDate"`
	userListIDSet                  bool
}

type antennaPayload struct {
	ID                             string     `json:"id"`
	CreatedAt                      string     `json:"createdAt"`
	Name                           string     `json:"name"`
	Keywords                       [][]string `json:"keywords"`
	ExcludeKeywords                [][]string `json:"excludeKeywords"`
	Source                         string     `json:"src"`
	UserListID                     *string    `json:"userListId"`
	Users                          []string   `json:"users"`
	CaseSensitive                  bool       `json:"caseSensitive"`
	LocalOnly                      bool       `json:"localOnly"`
	ExcludeBots                    bool       `json:"excludeBots"`
	WithReplies                    bool       `json:"withReplies"`
	WithFile                       bool       `json:"withFile"`
	IsActive                       bool       `json:"isActive"`
	ExcludeNotesInSensitiveChannel bool       `json:"excludeNotesInSensitiveChannel"`
	HasUnreadNote                  bool       `json:"hasUnreadNote"`
	Notify                         bool       `json:"notify"`
}

func (r *antennaJSON) UnmarshalJSON(data []byte) error {
	type alias antennaJSON
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*r = antennaJSON(decoded)
	_, r.userListIDSet = fields["userListId"]
	return nil
}

func (h *Handler) listAntennas(c *gin.Context) {
	if h.clients.UserAntennas == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "user service unavailable", "service_unavailable")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserAntennas.ListAntennas(ctx, &userpb.ListAntennasRequest{OwnerId: currentUserID(c)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := antennaPayloads(result.GetItems())
	if isAntennaCompatibilityRoute(c) {
		c.JSON(stdhttp.StatusOK, items)
		return
	}
	response.Success(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) createAntenna(c *gin.Context) {
	var req antennaJSON
	if !bindJSON(c, &req) {
		return
	}
	if req.Name == nil || req.Source == nil {
		writeError(c, stdhttp.StatusBadRequest, "name and src are required", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserAntennas.CreateAntenna(ctx, &userpb.CreateAntennaRequest{OwnerId: currentUserID(c), Name: *req.Name, Source: *req.Source, UserListId: antennaIDValue(req.UserListID), Keywords: antennaKeywordGroups(req.Keywords), ExcludeKeywords: antennaKeywordGroups(req.ExcludeKeywords), Users: antennaUsers(req.Users), CaseSensitive: boolValue(req.CaseSensitive), LocalOnly: boolValue(req.LocalOnly), ExcludeBots: boolValue(req.ExcludeBots), WithReplies: boolValue(req.WithReplies), WithFile: boolValue(req.WithFile), ExcludeNotesInSensitiveChannel: boolValue(req.ExcludeNotesInSensitiveChannel)})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.writeAntenna(c, result.GetAntenna())
}

func (h *Handler) showAntenna(c *gin.Context) {
	var req antennaJSON
	if c.Param("antennaId") == "" && !bindJSON(c, &req) {
		return
	}
	id, ok := antennaRequestID(c, req.AntennaID)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.clients.UserAntennas.GetAntenna(ctx, &userpb.GetAntennaRequest{OwnerId: currentUserID(c), AntennaId: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.writeAntenna(c, result.GetAntenna())
}

func (h *Handler) updateAntenna(c *gin.Context) {
	var req antennaJSON
	if !bindJSON(c, &req) {
		return
	}
	id, ok := antennaRequestID(c, req.AntennaID)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	current, err := h.clients.UserAntennas.GetAntenna(ctx, &userpb.GetAntennaRequest{OwnerId: currentUserID(c), AntennaId: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	base := current.GetAntenna()
	if base == nil {
		writeError(c, stdhttp.StatusNotFound, "antenna not found", "not_found")
		return
	}
	result, err := h.clients.UserAntennas.UpdateAntenna(ctx, &userpb.UpdateAntennaRequest{OwnerId: currentUserID(c), AntennaId: id, Name: stringOr(req.Name, base.GetName()), Source: stringOr(req.Source, base.GetSource()), UserListId: idOr(req.UserListID, req.userListIDSet, base.GetUserListId()), Keywords: groupsOr(req.Keywords, base.GetKeywords()), ExcludeKeywords: groupsOr(req.ExcludeKeywords, base.GetExcludeKeywords()), Users: usersOr(req.Users, base.GetUsers()), CaseSensitive: boolOr(req.CaseSensitive, base.GetCaseSensitive()), LocalOnly: boolOr(req.LocalOnly, base.GetLocalOnly()), ExcludeBots: boolOr(req.ExcludeBots, base.GetExcludeBots()), WithReplies: boolOr(req.WithReplies, base.GetWithReplies()), WithFile: boolOr(req.WithFile, base.GetWithFile()), ExcludeNotesInSensitiveChannel: boolOr(req.ExcludeNotesInSensitiveChannel, base.GetExcludeNotesInSensitiveChannel())})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	h.writeAntenna(c, result.GetAntenna())
}

func (h *Handler) deleteAntenna(c *gin.Context) {
	var req antennaJSON
	if c.Param("antennaId") == "" && !bindJSON(c, &req) {
		return
	}
	id, ok := antennaRequestID(c, req.AntennaID)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	if _, err := h.clients.UserAntennas.DeleteAntenna(ctx, &userpb.DeleteAntennaRequest{OwnerId: currentUserID(c), AntennaId: id}); err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) antennaNotes(c *gin.Context) {
	var req antennaJSON
	if isAntennaCompatibilityRoute(c) && !bindJSON(c, &req) {
		return
	}
	id, ok := antennaRequestID(c, req.AntennaID)
	if !ok {
		return
	}
	if h.clients.FeedFiltered == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "feed service unavailable", "service_unavailable")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	antenna, err := h.clients.UserAntennas.GetAntenna(ctx, &userpb.GetAntennaRequest{OwnerId: currentUserID(c), AntennaId: id})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	a := antenna.GetAntenna()
	if a == nil {
		writeError(c, stdhttp.StatusNotFound, "antenna not found", "not_found")
		return
	}
	authorIDs, excludedIDs, err := h.resolveAntennaAuthors(ctx, a)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	hidden, err := h.hiddenUserIDSet(ctx, currentUserID(c))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	for userID := range hidden {
		excludedIDs = append(excludedIDs, userID)
	}
	feedReq := &feedpb.FilteredFeedRequest{Limit: normalizeFeedLimit(antennaLimit(c, req.Limit)), Offset: normalizeFeedOffset(queryInt32(c, "offset", 0)), AuthorIds: authorIDs, ExcludedAuthorIds: excludedIDs, Keywords: feedKeywordGroups(a.GetKeywords()), ExcludeKeywords: feedKeywordGroups(a.GetExcludeKeywords()), CaseSensitive: a.GetCaseSensitive(), WithFile: a.GetWithFile(), RestrictAuthors: antennaRestrictsAuthors(a.GetSource())}
	feedReq.SinceId = antennaIDQuery(req.SinceID, c.Query("sinceId"))
	feedReq.UntilId = antennaIDQuery(req.UntilID, c.Query("untilId"))
	feedReq.SincePublishedAt = antennaDateQuery(req.SinceDate, c.Query("sinceDate"))
	feedReq.UntilPublishedAt = antennaDateQuery(req.UntilDate, c.Query("untilDate"))
	result, err := h.clients.FeedFiltered.ListFiltered(ctx, feedReq)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if isAntennaCompatibilityRoute(c) {
		c.JSON(stdhttp.StatusOK, result.GetItems())
		return
	}
	response.Success(c, result)
}

func (h *Handler) resolveAntennaAuthors(ctx context.Context, antenna *userpb.AntennaInfo) ([]int64, []int64, error) {
	var authors, excluded []int64
	switch antenna.GetSource() {
	case "home":
		following, err := h.followingIDSet(ctx, antenna.GetOwnerId())
		if err != nil {
			return nil, nil, err
		}
		for id := range following {
			authors = append(authors, id)
		}
		authors = append(authors, antenna.GetOwnerId())
	case "users":
		var err error
		authors, err = h.resolveAntennaUserIDs(ctx, antenna.GetUsers())
		if err != nil {
			return nil, nil, err
		}
	case "list":
		members, err := h.clients.UserLists.ListUserListMembers(ctx, &userpb.ListUserListMembersRequest{ViewerId: antenna.GetOwnerId(), ListId: antenna.GetUserListId(), Page: 1, PageSize: userListMaxPageSize})
		if err != nil {
			return nil, nil, err
		}
		for _, member := range members.GetItems() {
			if member != nil && member.GetId() > 0 {
				authors = append(authors, member.GetId())
			}
		}
	case "users_blacklist":
		var err error
		excluded, err = h.resolveAntennaUserIDs(ctx, antenna.GetUsers())
		if err != nil {
			return nil, nil, err
		}
	}
	sort.Slice(authors, func(i, j int) bool { return authors[i] < authors[j] })
	sort.Slice(excluded, func(i, j int) bool { return excluded[i] < excluded[j] })
	return uniqueInt64(authors), uniqueInt64(excluded), nil
}

func (h *Handler) resolveAntennaUserIDs(ctx context.Context, users []string) ([]int64, error) {
	ids := make([]int64, 0, len(users))
	localHost, _ := h.exportAccountHost()
	for _, raw := range users {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if id, err := strconv.ParseInt(value, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
			continue
		}
		username := strings.TrimPrefix(value, "@")
		if separator := strings.LastIndex(username, "@"); separator > 0 {
			if localHost == "" || !strings.EqualFold(username[separator+1:], localHost) {
				continue
			}
			username = username[:separator]
		}
		result, err := h.clients.User.GetUserByUsername(ctx, &userpb.UsernameRequest{Username: username})
		if status.Code(err) == codes.NotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		if result.GetUser() != nil && result.GetUser().GetId() > 0 {
			ids = append(ids, result.GetUser().GetId())
		}
	}
	return uniqueInt64(ids), nil
}

func antennaRestrictsAuthors(source string) bool {
	switch source {
	case "home", "users", "list":
		return true
	default:
		return false
	}
}

func (h *Handler) writeAntenna(c *gin.Context, antenna *userpb.AntennaInfo) {
	if antenna == nil {
		writeError(c, stdhttp.StatusInternalServerError, "user service returned an empty antenna", "internal_error")
		return
	}
	payload := antennaPayloadFromProto(antenna)
	if isAntennaCompatibilityRoute(c) {
		c.JSON(stdhttp.StatusOK, payload)
		return
	}
	response.Success(c, gin.H{"antenna": payload})
}

func antennaPayloads(items []*userpb.AntennaInfo) []antennaPayload {
	out := make([]antennaPayload, 0, len(items))
	for _, item := range items {
		if item != nil {
			out = append(out, antennaPayloadFromProto(item))
		}
	}
	return out
}
func antennaPayloadFromProto(a *userpb.AntennaInfo) antennaPayload {
	id := strconv.FormatInt(a.GetId(), 10)
	var listID *string
	if a.GetUserListId() > 0 {
		value := strconv.FormatInt(a.GetUserListId(), 10)
		listID = &value
	}
	return antennaPayload{ID: id, CreatedAt: formatUnixMilli(a.GetCreatedAt()), Name: a.GetName(), Keywords: keywordGroupsFromProto(a.GetKeywords()), ExcludeKeywords: keywordGroupsFromProto(a.GetExcludeKeywords()), Source: a.GetSource(), UserListID: listID, Users: a.GetUsers(), CaseSensitive: a.GetCaseSensitive(), LocalOnly: a.GetLocalOnly(), ExcludeBots: a.GetExcludeBots(), WithReplies: a.GetWithReplies(), WithFile: a.GetWithFile(), IsActive: a.GetIsActive(), ExcludeNotesInSensitiveChannel: a.GetExcludeNotesInSensitiveChannel()}
}
func keywordGroupsFromProto(groups []*userpb.AntennaKeywordGroup) [][]string {
	out := make([][]string, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			out = append(out, append([]string(nil), group.GetTerms()...))
		}
	}
	return out
}
func antennaKeywordGroups(groups *[][]string) []*userpb.AntennaKeywordGroup {
	if groups == nil {
		return nil
	}
	out := make([]*userpb.AntennaKeywordGroup, 0, len(*groups))
	for _, group := range *groups {
		out = append(out, &userpb.AntennaKeywordGroup{Terms: append([]string(nil), group...)})
	}
	return out
}
func feedKeywordGroups(groups []*userpb.AntennaKeywordGroup) []*feedpb.FeedKeywordGroup {
	out := make([]*feedpb.FeedKeywordGroup, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			out = append(out, &feedpb.FeedKeywordGroup{Terms: append([]string(nil), group.GetTerms()...)})
		}
	}
	return out
}
func antennaUsers(users *[]string) []string {
	if users == nil {
		return nil
	}
	return append([]string(nil), (*users)...)
}
func boolValue(value *bool) bool { return value != nil && *value }
func stringOr(value *string, fallback string) string {
	if value != nil {
		return *value
	}
	return fallback
}
func boolOr(value *bool, fallback bool) bool {
	if value != nil {
		return *value
	}
	return fallback
}
func groupsOr(value *[][]string, fallback []*userpb.AntennaKeywordGroup) []*userpb.AntennaKeywordGroup {
	if value != nil {
		return antennaKeywordGroups(value)
	}
	return append([]*userpb.AntennaKeywordGroup(nil), fallback...)
}
func usersOr(value *[]string, fallback []string) []string {
	if value != nil {
		return antennaUsers(value)
	}
	return append([]string(nil), fallback...)
}
func idOr(value *string, present bool, fallback int64) int64 {
	if !present {
		return fallback
	}
	return antennaIDValue(value)
}
func antennaIDValue(value *string) int64 {
	if value == nil {
		return 0
	}
	id, _ := strconv.ParseInt(strings.TrimSpace(*value), 10, 64)
	return id
}
func antennaRequestID(c *gin.Context, body string) (int64, bool) {
	raw := strings.TrimSpace(c.Param("antennaId"))
	if raw == "" {
		raw = strings.TrimSpace(body)
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "antennaId must be a positive integer", "bad_request")
		return 0, false
	}
	return id, true
}
func isAntennaCompatibilityRoute(c *gin.Context) bool {
	path := c.FullPath()
	return strings.HasPrefix(path, "/api/i/") || strings.HasPrefix(path, "/i/") || strings.HasPrefix(path, "/api/antennas/") || strings.HasPrefix(path, "/antennas/")
}
func antennaLimit(c *gin.Context, body int32) int32 {
	if body > 0 {
		return body
	}
	return queryInt32(c, "limit", 20)
}
func antennaIDQuery(body, query string) int64 {
	raw := strings.TrimSpace(body)
	if raw == "" {
		raw = strings.TrimSpace(query)
	}
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}
func antennaDateQuery(body int64, query string) int64 {
	if body > 0 {
		return body
	}
	value, _ := strconv.ParseInt(strings.TrimSpace(query), 10, 64)
	return value
}
func uniqueInt64(values []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value > 0 {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				out = append(out, value)
			}
		}
	}
	return out
}
