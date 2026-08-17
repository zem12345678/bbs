package http

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/url"
	"strings"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/idna"
)

const antennaExportMemberPageSize = int32(100)

type antennaExportRecord struct {
	Name            string     `json:"name"`
	Source          string     `json:"src"`
	Keywords        [][]string `json:"keywords"`
	ExcludeKeywords [][]string `json:"excludeKeywords"`
	Users           []string   `json:"users"`
	UserListAccts   []string   `json:"userListAccts"`
	CaseSensitive   bool       `json:"caseSensitive"`
	LocalOnly       bool       `json:"localOnly"`
	ExcludeBots     bool       `json:"excludeBots"`
	WithReplies     bool       `json:"withReplies"`
	WithFile        bool       `json:"withFile"`
}

func (h *Handler) exportAntennas(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.UserAntennas == nil || h.clients.UserLists == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "antenna export dependencies unavailable", "service_unavailable")
		return
	}
	h.deliverUserExport(c, userExportSpec{
		label: "antenna", filenamePrefix: "antennas", exportedEntity: "antenna",
		gate: h.antennaExportGate, build: h.buildAntennaExport,
	})
}

func (h *Handler) buildAntennaExport(ctx context.Context, userID int64) ([]byte, error) {
	response, err := h.clients.UserAntennas.ListAntennas(ctx, &userpb.ListAntennasRequest{OwnerId: userID})
	if err != nil {
		return nil, err
	}
	records := make([]antennaExportRecord, 0, len(response.GetItems()))
	for _, antenna := range response.GetItems() {
		record := antennaExportRecord{
			Name: antenna.GetName(), Source: antenna.GetSource(),
			Keywords:        antennaExportKeywordGroups(antenna.GetKeywords()),
			ExcludeKeywords: antennaExportKeywordGroups(antenna.GetExcludeKeywords()),
			Users:           append([]string{}, antenna.GetUsers()...),
			CaseSensitive:   antenna.GetCaseSensitive(), LocalOnly: antenna.GetLocalOnly(),
			ExcludeBots: antenna.GetExcludeBots(), WithReplies: antenna.GetWithReplies(), WithFile: antenna.GetWithFile(),
		}
		if antenna.GetUserListId() > 0 {
			record.UserListAccts, err = h.antennaUserListAccounts(ctx, userID, antenna.GetUserListId())
			if err != nil {
				return nil, err
			}
		}
		records = append(records, record)
	}
	return json.Marshal(records)
}

func antennaExportKeywordGroups(groups []*userpb.AntennaKeywordGroup) [][]string {
	result := make([][]string, 0, len(groups))
	for _, group := range groups {
		result = append(result, append([]string{}, group.GetTerms()...))
	}
	return result
}

func (h *Handler) antennaUserListAccounts(ctx context.Context, userID, listID int64) ([]string, error) {
	host, err := h.exportAccountHost()
	if err != nil {
		return nil, err
	}
	accounts := make([]string, 0)
	for page := int32(1); ; page++ {
		response, err := h.clients.UserLists.ListUserListMembers(ctx, &userpb.ListUserListMembersRequest{
			ViewerId: userID, ListId: listID, Page: page, PageSize: antennaExportMemberPageSize,
		})
		if err != nil {
			return nil, err
		}
		for _, member := range response.GetItems() {
			accounts = append(accounts, member.GetUsername()+"@"+host)
		}
		if len(response.GetItems()) < int(antennaExportMemberPageSize) || int64(len(accounts)) >= response.GetTotal() {
			break
		}
	}
	return accounts, nil
}

func (h *Handler) exportAccountHost() (string, error) {
	parsed, err := url.Parse(h.publicBaseURL)
	if err != nil || parsed.Hostname() == "" {
		return "", errors.New("public base URL host unavailable")
	}
	host, err := idna.Lookup.ToASCII(strings.ToLower(parsed.Hostname()))
	if err != nil || host == "" {
		return "", errors.New("public base URL host unavailable")
	}
	return host, nil
}
