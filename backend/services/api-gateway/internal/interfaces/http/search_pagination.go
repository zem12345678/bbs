package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	defaultSearchPage     int32 = 1
	defaultSearchPageSize int32 = 20
	maxSearchPageSize     int32 = 100
	maxSearchResultWindow       = int64(10000)
)

func searchPagination(c *gin.Context) (int32, int32, bool) {
	page, ok := searchPositiveInt32Query(c, "page", defaultSearchPage, 0)
	if !ok {
		return 0, 0, false
	}
	pageSize, ok := searchPositiveInt32Query(c, "page_size", defaultSearchPageSize, maxSearchPageSize)
	if !ok {
		return 0, 0, false
	}
	if (int64(page)-1)*int64(pageSize)+int64(pageSize) > maxSearchResultWindow {
		writeError(c, http.StatusBadRequest, "search pagination exceeds maximum result window", "bad_request")
		return 0, 0, false
	}
	return page, pageSize, true
}

func searchPositiveInt32Query(c *gin.Context, name string, fallback int32, maximum int32) (int32, bool) {
	raw, exists := c.GetQuery(name)
	if !exists {
		return fallback, true
	}
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
	if err != nil || value <= 0 {
		writeError(c, http.StatusBadRequest, name+" must be a positive integer", "bad_request")
		return 0, false
	}
	if maximum > 0 && value > int64(maximum) {
		writeError(c, http.StatusBadRequest, name+" must not exceed "+strconv.FormatInt(int64(maximum), 10), "bad_request")
		return 0, false
	}
	return int32(value), true
}
