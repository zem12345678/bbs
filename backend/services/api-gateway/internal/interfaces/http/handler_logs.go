package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/adminpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const maxLoggedRequestBody = 4096
const redactedLogValue = "[REDACTED]"

func (h *Handler) listLoginLogs(c *gin.Context) {
	page, pageSize := systemPage(c)
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListLoginLogs(ctx, &adminpb.ListLoginLogsRequest{
		Actor:  currentActor(c),
		Status: queryLogStatus(c),
		Query:  firstQuery(c, "query", "username", "ip"),
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, systemTablePayload(toHTTPLoginLogs(resp.GetItems()), resp.GetTotal(), page, pageSize))
}

func (h *Handler) listOperationLogs(c *gin.Context) {
	page, pageSize := systemPage(c)
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListOperationLogs(ctx, &adminpb.ListOperationLogsRequest{
		Actor:  currentActor(c),
		Status: queryLogStatus(c),
		Query:  firstQuery(c, "query", "module", "username", "ip"),
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, systemTablePayload(toHTTPOperationLogs(resp.GetItems()), resp.GetTotal(), page, pageSize))
}

func captureAdminRequestBody(c *gin.Context) string {
	if c.Request == nil || c.Request.Body == nil || c.Request.Method == http.MethodGet {
		return ""
	}
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return "[multipart/form-data]"
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(nil))
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return sanitizeLoggedRequestBody(contentType, body)
}

func sanitizeLoggedRequestBody(contentType string, body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	if isJSONContentType(contentType) || trimmed[0] == '{' || trimmed[0] == '[' {
		if value, ok := redactJSONRequestBody(trimmed); ok {
			return truncateLoggedString(value)
		}
	}
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		if values, err := url.ParseQuery(string(body)); err == nil {
			for key := range values {
				if isSensitiveLogKey(key) {
					values[key] = []string{redactedLogValue}
				}
			}
			return truncateLoggedString(values.Encode())
		}
	}
	return truncateLoggedString(string(body))
}

func isJSONContentType(contentType string) bool {
	return strings.Contains(contentType, "application/json") || strings.Contains(contentType, "+json")
}

func redactJSONRequestBody(body []byte) (string, bool) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", false
	}
	redactLogValue(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

func redactLogValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		if shouldRedactSettingPayloadValue(typed) {
			redactMapValueFields(typed)
		}
		for key, item := range typed {
			if isSensitiveLogKey(key) {
				typed[key] = redactedLogValue
				continue
			}
			redactLogValue(item)
		}
	case []any:
		for _, item := range typed {
			redactLogValue(item)
		}
	}
}

func shouldRedactSettingPayloadValue(value map[string]any) bool {
	settingKey := stringValueByNormalizedKey(value, "key")
	valueType := stringValueByNormalizedKey(value, "valuetype")
	return isSensitiveLogKey(settingKey) || strings.EqualFold(strings.TrimSpace(valueType), "password")
}

func redactMapValueFields(value map[string]any) {
	for key := range value {
		if normalizeLogKey(key) == "value" {
			value[key] = redactedLogValue
		}
	}
}

func stringValueByNormalizedKey(value map[string]any, normalizedKey string) string {
	for key, item := range value {
		if normalizeLogKey(key) != normalizedKey {
			continue
		}
		if text, ok := item.(string); ok {
			return text
		}
	}
	return ""
}

func isSensitiveLogKey(key string) bool {
	normalized := normalizeLogKey(key)
	switch {
	case normalized == "pwd" || strings.HasSuffix(normalized, "pwd"):
		return true
	case strings.Contains(normalized, "password"):
		return true
	case strings.Contains(normalized, "passwd"):
		return true
	case strings.Contains(normalized, "token"):
		return true
	case strings.Contains(normalized, "secret"):
		return true
	case strings.Contains(normalized, "apikey"):
		return true
	case strings.Contains(normalized, "authorization"):
		return true
	case strings.Contains(normalized, "credential"):
		return true
	case strings.Contains(normalized, "privatekey"):
		return true
	default:
		return false
	}
}

func normalizeLogKey(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	return strings.NewReplacer("_", "", "-", "", ".", "").Replace(normalized)
}

func truncateLoggedString(value string) string {
	if len(value) <= maxLoggedRequestBody {
		return value
	}
	return value[:maxLoggedRequestBody] + "...[truncated]"
}

func (h *Handler) recordAdminOperationLog(c *gin.Context, started time.Time, requestBody string) {
	if !shouldRecordAdminOperation(c) {
		return
	}
	actor := currentActor(c)
	status := int32(1)
	if c.Writer.Status() >= http.StatusBadRequest {
		status = 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = h.clients.Admin.RecordOperationLog(ctx, &adminpb.RecordOperationLogRequest{
		Actor:         actor,
		Title:         adminOperationTitle(c.FullPath(), c.Request.URL.Path),
		BusinessType:  adminOperationBusinessType(c.Request.Method),
		Method:        c.FullPath(),
		RequestMethod: c.Request.Method,
		OperatorType:  "admin",
		Url:           c.Request.URL.RequestURI(),
		Ip:            c.ClientIP(),
		Params:        adminOperationParams(c, requestBody),
		Status:        status,
		Result:        strconv.Itoa(c.Writer.Status()),
		LatencyTime:   fmt.Sprintf("%dms", time.Since(started).Milliseconds()),
		UserAgent:     c.Request.UserAgent(),
	})
}

func shouldRecordAdminOperation(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
		return false
	}
	path := c.Request.URL.Path
	return strings.HasPrefix(path, "/api/v1/admin/")
}

func adminOperationParams(c *gin.Context, requestBody string) string {
	if strings.TrimSpace(requestBody) != "" {
		return requestBody
	}
	if c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.RawQuery
	}
	return ""
}

func adminOperationBusinessType(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	default:
		return strings.ToLower(method)
	}
}

func adminOperationTitle(fullPath string, path string) string {
	value := strings.ToLower(fullPath)
	if value == "" {
		value = strings.ToLower(path)
	}
	switch {
	case strings.Contains(value, "/system/users"):
		return "系统用户"
	case strings.Contains(value, "/system/roles"):
		return "系统角色"
	case strings.Contains(value, "/system/menus"):
		return "系统菜单"
	case strings.Contains(value, "/system/depts") || strings.Contains(value, "/system/departments"):
		return "系统部门"
	case strings.Contains(value, "/articles"):
		return "文章管理"
	case strings.Contains(value, "/topics"):
		return "话题管理"
	case strings.Contains(value, "/comments"):
		return "评论管理"
	case strings.Contains(value, "/users"):
		return "用户治理"
	case strings.Contains(value, "/forbidden-words"):
		return "敏感词"
	case strings.Contains(value, "/settings"):
		return "系统配置"
	default:
		return "管理后台"
	}
}

func toHTTPLoginLogs(items []*adminpb.LoginLogInfo) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{
			"id":         item.GetId(),
			"username":   item.GetUsername(),
			"status":     item.GetStatus(),
			"ip":         item.GetIp(),
			"address":    item.GetLocation(),
			"location":   item.GetLocation(),
			"browser":    item.GetBrowser(),
			"system":     item.GetOs(),
			"os":         item.GetOs(),
			"platform":   item.GetPlatform(),
			"behavior":   item.GetMessage(),
			"message":    item.GetMessage(),
			"remark":     item.GetRemark(),
			"loginTime":  item.GetLoginTime(),
			"login_time": item.GetLoginTime(),
		})
	}
	return out
}

func toHTTPOperationLogs(items []*adminpb.OperationLogInfo) []gin.H {
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		ua := httpUserAgentInfo(item.GetUserAgent())
		out = append(out, gin.H{
			"id":             item.GetId(),
			"username":       item.GetOperatorName(),
			"operatorName":   item.GetOperatorName(),
			"module":         item.GetTitle(),
			"title":          item.GetTitle(),
			"summary":        item.GetBusinessType(),
			"businessType":   item.GetBusinessType(),
			"method":         item.GetRequestMethod(),
			"handler":        item.GetMethod(),
			"ip":             item.GetIp(),
			"address":        item.GetLocation(),
			"location":       item.GetLocation(),
			"status":         item.GetStatus(),
			"url":            item.GetUrl(),
			"params":         item.GetParams(),
			"result":         item.GetResult(),
			"takesTime":      parseLatencyMillis(item.GetLatencyTime()),
			"latencyTime":    item.GetLatencyTime(),
			"userAgent":      item.GetUserAgent(),
			"browser":        ua.browser,
			"system":         ua.os,
			"operatingTime":  item.GetOperationTime(),
			"operation_time": item.GetOperationTime(),
		})
	}
	return out
}

func parseLatencyMillis(value string) int64 {
	value = strings.TrimSpace(strings.TrimSuffix(value, "ms"))
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func queryLogStatus(c *gin.Context) int32 {
	value := strings.TrimSpace(c.Query("status"))
	if value == "" {
		return -1
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return -1
	}
	return int32(parsed)
}

type httpUAInfo struct {
	browser string
	os      string
}

func httpUserAgentInfo(value string) httpUAInfo {
	lower := strings.ToLower(value)
	info := httpUAInfo{browser: "Unknown", os: "Unknown"}
	switch {
	case strings.Contains(lower, "edg/"):
		info.browser = "Edge"
	case strings.Contains(lower, "chrome/"):
		info.browser = "Chrome"
	case strings.Contains(lower, "firefox/"):
		info.browser = "Firefox"
	case strings.Contains(lower, "safari/"):
		info.browser = "Safari"
	}
	switch {
	case strings.Contains(lower, "windows"):
		info.os = "Windows"
	case strings.Contains(lower, "mac os"):
		info.os = "macOS"
	case strings.Contains(lower, "android"):
		info.os = "Android"
	case strings.Contains(lower, "iphone") || strings.Contains(lower, "ipad"):
		info.os = "iOS"
	case strings.Contains(lower, "linux"):
		info.os = "Linux"
	}
	return info
}
