package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"api-gateway/internal/clients/pb/adminpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const maxLoggedRequestBody = 4096

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
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxLoggedRequestBody+1))
	if err != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(nil))
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) > maxLoggedRequestBody {
		return string(body[:maxLoggedRequestBody]) + "...[truncated]"
	}
	return string(body)
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
