package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strings"
	"unicode/utf8"

	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
)

const (
	antennaImportMaxBytes = int64(2 << 20)
	antennaImportMaxItems = 20
)

func (h *Handler) importAntennas(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.File == nil || h.clients.UserAntennas == nil || h.attachments == nil {
		writeError(c, stdhttp.StatusServiceUnavailable, "antenna import dependencies unavailable", "service_unavailable")
		return
	}

	fileID, ok := bindImportFileID(c)
	if !ok {
		return
	}
	ownerID := currentUserID(c)
	if !h.allowAntennaImportRateLimit(c, ownerID) {
		return
	}

	ctx, cancel := rpcContext(c)
	defer cancel()
	payload, ok := h.readOwnedImportFile(c, ctx, ownerID, fileID, antennaImportMaxBytes)
	if !ok {
		return
	}

	var records []antennaExportRecord
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(&records); err != nil || records == nil {
		writeError(c, stdhttp.StatusBadRequest, "invalid antenna import file", "bad_request")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(c, stdhttp.StatusBadRequest, "invalid antenna import file", "bad_request")
		return
	}

	createRequests := antennaImportCreateRequests(ownerID, records)
	if len(createRequests) == 0 {
		c.Status(stdhttp.StatusNoContent)
		return
	}
	if len(createRequests) > antennaImportMaxItems {
		writeError(c, stdhttp.StatusPreconditionFailed, "antenna limit reached", "antenna_limit_reached")
		return
	}
	existing, err := h.clients.UserAntennas.ListAntennas(ctx, &userpb.ListAntennasRequest{OwnerId: ownerID})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if len(existing.GetItems())+len(createRequests) > antennaImportMaxItems {
		writeError(c, stdhttp.StatusPreconditionFailed, "antenna limit reached", "antenna_limit_reached")
		return
	}

	for _, createRequest := range createRequests {
		if _, err := h.clients.UserAntennas.CreateAntenna(ctx, createRequest); err != nil {
			writeRPCError(c, err)
			return
		}
	}
	c.Status(stdhttp.StatusNoContent)
}

func antennaImportCreateRequests(ownerID int64, records []antennaExportRecord) []*userpb.CreateAntennaRequest {
	requests := make([]*userpb.CreateAntennaRequest, 0, len(records))
	for _, record := range records {
		name := strings.TrimSpace(record.Name)
		source := strings.ToLower(strings.TrimSpace(record.Source))
		users := append([]string(nil), record.Users...)
		if source == "list" {
			if !hasAntennaImportUsers(record.UserListAccts) {
				continue
			}
			source = "users"
			users = append([]string(nil), record.UserListAccts...)
		}
		if name == "" || utf8.RuneCountInString(name) > 100 || !validAntennaImportSource(source) ||
			(!hasAntennaImportKeywords(record.Keywords) && !hasAntennaImportKeywords(record.ExcludeKeywords)) ||
			(source == "users" && !hasAntennaImportUsers(users)) {
			continue
		}
		requests = append(requests, &userpb.CreateAntennaRequest{
			OwnerId: ownerID, Name: name, Source: source,
			Keywords: antennaKeywordGroups(&record.Keywords), ExcludeKeywords: antennaKeywordGroups(&record.ExcludeKeywords),
			Users: append([]string(nil), users...), CaseSensitive: record.CaseSensitive, LocalOnly: record.LocalOnly,
			ExcludeBots: record.ExcludeBots, WithReplies: record.WithReplies, WithFile: record.WithFile,
		})
	}
	return requests
}

func validAntennaImportSource(source string) bool {
	switch source {
	case "home", "all", "users", "users_blacklist":
		return true
	default:
		return false
	}
}

func hasAntennaImportKeywords(groups [][]string) bool {
	for _, group := range groups {
		for _, term := range group {
			if strings.TrimSpace(term) != "" {
				return true
			}
		}
	}
	return false
}

func hasAntennaImportUsers(users []string) bool {
	for _, user := range users {
		if strings.TrimSpace(user) != "" {
			return true
		}
	}
	return false
}
