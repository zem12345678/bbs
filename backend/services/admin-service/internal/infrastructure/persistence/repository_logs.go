package persistence

import (
	"context"
	"strconv"
	"strings"
	"time"

	domain "admin/internal/domain/admin"
	"admin/internal/infrastructure/persistence/po"
)

func (r *Repository) ListLoginLogs(ctx context.Context, status int32, query string, limit int32, offset int32) (domain.LoginLogList, error) {
	limit, offset = normalizePage(limit, offset)
	db := r.db.WithContext(ctx).Model(&po.LoginLog{})
	if status >= 0 {
		db = db.Where("status = ?", logStatus(status))
	}
	query = normalize(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("LOWER(username) LIKE ? OR LOWER(ipaddr) LIKE ? OR LOWER(msg) LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.LoginLogList{}, err
	}
	var rows []po.LoginLog
	if err := db.Order("login_time DESC, id DESC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return domain.LoginLogList{}, err
	}
	items := make([]domain.LoginLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainLoginLog(row))
	}
	return domain.LoginLogList{Items: items, Total: total}, nil
}

func (r *Repository) RecordLoginLog(ctx context.Context, command domain.RecordLoginLogCommand) error {
	now := time.Now()
	ua := parseUserAgent(command.UserAgent)
	row := po.LoginLog{
		Username:  strings.TrimSpace(command.Username),
		Status:    logStatus(command.Status),
		Ipaddr:    strings.TrimSpace(command.IP),
		Browser:   ua.browser,
		Os:        ua.os,
		Platform:  ua.platform,
		LoginTime: now,
		Msg:       strings.TrimSpace(command.Message),
		Remark:    strings.TrimSpace(command.Remark),
	}
	row.CreateTime = now
	row.UpdateTime = now
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *Repository) CountRecentLoginFailures(ctx context.Context, username string, ip string, since time.Time) (int64, error) {
	username = normalize(username)
	if username == "" {
		return 0, nil
	}
	db := r.db.WithContext(ctx).Model(&po.LoginLog{}).
		Where("status = ?", logStatus(0)).
		Where("LOWER(username) = ?", username).
		Where("login_time >= ?", since)
	ip = strings.TrimSpace(ip)
	if ip != "" {
		db = db.Where("ipaddr = ?", ip)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) ListOperationLogs(ctx context.Context, status int32, query string, limit int32, offset int32) (domain.OperationLogList, error) {
	limit, offset = normalizePage(limit, offset)
	db := r.db.WithContext(ctx).Model(&po.SysOperaLog{})
	if status >= 0 {
		db = db.Where("status = ?", logStatus(status))
	}
	query = normalize(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("LOWER(title) LIKE ? OR LOWER(oper_name) LIKE ? OR LOWER(oper_url) LIKE ? OR LOWER(request_method) LIKE ? OR LOWER(oper_ip) LIKE ? OR LOWER(oper_param) LIKE ? OR LOWER(json_result) LIKE ? OR LOWER(remark) LIKE ?", like, like, like, like, like, like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return domain.OperationLogList{}, err
	}
	var rows []po.SysOperaLog
	if err := db.Order("oper_time DESC, id DESC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return domain.OperationLogList{}, err
	}
	items := make([]domain.OperationLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomainOperationLog(row))
	}
	return domain.OperationLogList{Items: items, Total: total}, nil
}

func (r *Repository) RecordOperationLog(ctx context.Context, command domain.RecordOperationLogCommand) error {
	now := time.Now()
	row := po.SysOperaLog{
		Title:         strings.TrimSpace(command.Title),
		BusinessType:  strings.TrimSpace(command.BusinessType),
		Method:        strings.TrimSpace(command.Method),
		RequestMethod: strings.TrimSpace(command.RequestMethod),
		OperatorType:  strings.TrimSpace(command.OperatorType),
		OperName:      strings.TrimSpace(command.OperatorName),
		OperUrl:       strings.TrimSpace(command.URL),
		OperIp:        strings.TrimSpace(command.IP),
		OperParam:     strings.TrimSpace(command.Params),
		Status:        logStatus(command.Status),
		OperTime:      now,
		JsonResult:    strings.TrimSpace(command.Result),
		Remark:        strings.TrimSpace(command.Remark),
		LatencyTime:   strings.TrimSpace(command.LatencyTime),
		UserAgent:     strings.TrimSpace(command.UserAgent),
	}
	row.CreateTime = now
	row.UpdateTime = now
	return r.db.WithContext(ctx).Create(&row).Error
}

func toDomainLoginLog(log po.LoginLog) domain.LoginLog {
	return domain.LoginLog{
		ID:        log.ID,
		Username:  log.Username,
		Status:    parseLogStatus(log.Status),
		IP:        log.Ipaddr,
		Location:  log.LoginLocation,
		Browser:   log.Browser,
		OS:        log.Os,
		Platform:  log.Platform,
		Message:   log.Msg,
		Remark:    log.Remark,
		LoginTime: timeMillis(log.LoginTime),
	}
}

func toDomainOperationLog(log po.SysOperaLog) domain.OperationLog {
	return domain.OperationLog{
		ID:            log.ID,
		Title:         log.Title,
		BusinessType:  log.BusinessType,
		Method:        log.Method,
		RequestMethod: log.RequestMethod,
		OperatorType:  log.OperatorType,
		OperatorName:  log.OperName,
		DeptName:      log.DeptName,
		URL:           log.OperUrl,
		IP:            log.OperIp,
		Location:      log.OperLocation,
		Params:        log.OperParam,
		Status:        parseLogStatus(log.Status),
		OperationTime: timeMillis(log.OperTime),
		Result:        log.JsonResult,
		Remark:        log.Remark,
		LatencyTime:   log.LatencyTime,
		UserAgent:     log.UserAgent,
	}
}

func logStatus(status int32) string {
	if status == 1 {
		return "1"
	}
	return "0"
}

func parseLogStatus(value string) int32 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
	if err != nil {
		return 0
	}
	if parsed == 1 {
		return 1
	}
	return 0
}

type userAgentInfo struct {
	browser  string
	os       string
	platform string
}

func parseUserAgent(value string) userAgentInfo {
	lower := strings.ToLower(value)
	info := userAgentInfo{browser: "Unknown", os: "Unknown", platform: "Web"}
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
	if strings.Contains(lower, "mobile") {
		info.platform = "Mobile"
	}
	return info
}
