package http

import (
	"context"
	"fmt"
	"sort"
	"time"

	"api-gateway/api/proto/adminpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

const overviewSampleLimit int32 = 100

func (h *Handler) adminOverview(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	payload, err := h.buildAdminOverview(ctx, currentActor(c))
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, payload)
}

func (h *Handler) buildAdminOverview(ctx context.Context, actor *adminpb.Actor) (gin.H, error) {
	users, err := h.clients.Admin.ListUsers(ctx, &adminpb.ListUsersRequest{Actor: actor, Page: 1, PageSize: overviewSampleLimit})
	if err != nil {
		return nil, err
	}
	articles, err := h.clients.Admin.ListArticles(ctx, &adminpb.ListArticlesRequest{Actor: actor, Limit: overviewSampleLimit})
	if err != nil {
		return nil, err
	}
	hiddenArticles, err := h.clients.Admin.ListArticles(ctx, &adminpb.ListArticlesRequest{Actor: actor, Status: 3, Limit: 1})
	if err != nil {
		return nil, err
	}
	topics, err := h.clients.Admin.ListTopics(ctx, &adminpb.ListTopicsRequest{Actor: actor, Limit: overviewSampleLimit})
	if err != nil {
		return nil, err
	}
	hiddenTopics, err := h.clients.Admin.ListTopics(ctx, &adminpb.ListTopicsRequest{Actor: actor, Status: 3, Limit: 1})
	if err != nil {
		return nil, err
	}
	comments, err := h.clients.Admin.ListComments(ctx, &adminpb.ListCommentsRequest{Actor: actor, Status: -1, Page: 1, PageSize: overviewSampleLimit})
	if err != nil {
		return nil, err
	}
	hiddenComments, err := h.clients.Admin.ListComments(ctx, &adminpb.ListCommentsRequest{Actor: actor, Status: 0, Page: 1, PageSize: 1})
	if err != nil {
		return nil, err
	}
	reports, err := h.clients.Admin.ListReports(ctx, &adminpb.ListReportsRequest{Actor: actor, Limit: overviewSampleLimit})
	if err != nil {
		return nil, err
	}
	pendingReports, err := h.clients.Admin.ListReports(ctx, &adminpb.ListReportsRequest{Actor: actor, Status: 1, Limit: overviewSampleLimit})
	if err != nil {
		return nil, err
	}
	loginLogs, err := h.clients.Admin.ListLoginLogs(ctx, &adminpb.ListLoginLogsRequest{Actor: actor, Status: -1, Limit: 10})
	if err != nil {
		return nil, err
	}
	operationLogs, err := h.clients.Admin.ListOperationLogs(ctx, &adminpb.ListOperationLogsRequest{Actor: actor, Status: -1, Limit: 10})
	if err != nil {
		return nil, err
	}

	labels14, keys14 := overviewDayKeys(14)
	labels7 := labels14[7:]
	usersSeries14 := overviewUserSeries(keys14, users.GetItems())
	articlesSeries14 := overviewArticleSeries(keys14, articles.GetItems())
	topicsSeries14 := overviewTopicSeries(keys14, topics.GetItems())
	commentsSeries14 := overviewCommentSeries(keys14, comments.GetItems())
	reportsSeries14 := overviewReportSeries(keys14, reports.GetItems())
	pendingSeries14 := overviewReportSeries(keys14, pendingReports.GetItems())

	contentSeries14 := addIntSeries(articlesSeries14, topicsSeries14, commentsSeries14)
	governanceSeries14 := addIntSeries(reportsSeries14, pendingSeries14)
	contentSeries7 := contentSeries14[7:]
	usersSeries7 := usersSeries14[7:]
	commentsSeries7 := commentsSeries14[7:]
	pendingSeries7 := pendingSeries14[7:]

	totalContent := articles.GetTotal() + topics.GetTotal()
	hiddenContent := hiddenArticles.GetTotal() + hiddenTopics.GetTotal()
	return gin.H{
		"metrics": []gin.H{
			overviewMetric("users", "注册用户", users.GetTotal(), usersSeries7, "本期新增 "+formatInt(sumInts(usersSeries7))),
			overviewMetric("content", "内容总量", totalContent, contentSeries7, "本期新增 "+formatInt(sumInts(contentSeries7))),
			overviewMetric("comments", "评论总量", comments.GetTotal(), commentsSeries7, "本期新增 "+formatInt(sumInts(commentsSeries7))),
			overviewMetric("pending_reports", "待处理举报", pendingReports.GetTotal(), pendingSeries7, overviewRateText(pendingReports.GetTotal(), reports.GetTotal())),
		},
		"chart": gin.H{
			"labels": labels7,
			"previous": gin.H{
				"contentData":    contentSeries14[:7],
				"governanceData": governanceSeries14[:7],
			},
			"current": gin.H{
				"contentData":    contentSeries7,
				"governanceData": governanceSeries14[7:],
			},
		},
		"progress": []gin.H{
			overviewProgress("举报处理率", reports.GetTotal()-pendingReports.GetTotal(), reports.GetTotal(), "#26ce83"),
			overviewProgress("内容正常率", totalContent-hiddenContent, totalContent, "#41b6ff"),
			overviewProgress("评论可见率", comments.GetTotal()-hiddenComments.GetTotal(), comments.GetTotal(), "#7846e5"),
		},
		"daily":  overviewDailyRows(labels14, usersSeries14, articlesSeries14, topicsSeries14, commentsSeries14, reportsSeries14, pendingSeries14),
		"latest": overviewActivities(loginLogs.GetItems(), operationLogs.GetItems()),
	}, nil
}

func overviewMetric(key string, name string, value int64, data []int, percent string) gin.H {
	return gin.H{"key": key, "name": name, "value": value, "data": data, "percent": percent}
}

func overviewProgress(label string, numerator int64, denominator int64, color string) gin.H {
	percentage := int64(100)
	if denominator > 0 {
		percentage = numerator * 100 / denominator
		if percentage < 0 {
			percentage = 0
		}
		if percentage > 100 {
			percentage = 100
		}
	}
	return gin.H{"label": label, "week": label, "percentage": percentage, "duration": 100, "color": color}
}

func overviewRateText(value int64, total int64) string {
	if total <= 0 {
		return "暂无待处理"
	}
	return fmt.Sprintf("占比 %d%%", value*100/total)
}

func overviewDayKeys(days int) ([]string, map[string]int) {
	labels := make([]string, days)
	keys := make(map[string]int, days)
	now := time.Now()
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -(days - 1 - i))
		key := day.Format("2006-01-02")
		labels[i] = key
		keys[key] = i
	}
	return labels, keys
}

func overviewUserSeries(keys map[string]int, items []*adminpb.UserInfo) []int {
	series := make([]int, len(keys))
	for _, item := range items {
		bumpOverviewSeries(series, keys, item.GetCreatedAt())
	}
	return series
}

func overviewArticleSeries(keys map[string]int, items []*adminpb.ArticleInfo) []int {
	series := make([]int, len(keys))
	for _, item := range items {
		bumpOverviewSeries(series, keys, item.GetCreatedAt())
	}
	return series
}

func overviewTopicSeries(keys map[string]int, items []*adminpb.TopicInfo) []int {
	series := make([]int, len(keys))
	for _, item := range items {
		bumpOverviewSeries(series, keys, item.GetCreatedAt())
	}
	return series
}

func overviewCommentSeries(keys map[string]int, items []*adminpb.CommentInfo) []int {
	series := make([]int, len(keys))
	for _, item := range items {
		bumpOverviewSeries(series, keys, item.GetCreatedAt())
	}
	return series
}

func overviewReportSeries(keys map[string]int, items []*adminpb.ReportInfo) []int {
	series := make([]int, len(keys))
	for _, item := range items {
		bumpOverviewSeries(series, keys, item.GetCreatedAt())
	}
	return series
}

func bumpOverviewSeries(series []int, keys map[string]int, timestamp int64) {
	value := overviewTime(timestamp)
	if value.IsZero() {
		return
	}
	index, ok := keys[value.Format("2006-01-02")]
	if !ok || index < 0 || index >= len(series) {
		return
	}
	series[index]++
}

func overviewTime(timestamp int64) time.Time {
	if timestamp <= 0 {
		return time.Time{}
	}
	if timestamp < 1_000_000_000_000 {
		return time.Unix(timestamp, 0)
	}
	return time.UnixMilli(timestamp)
}

func addIntSeries(series ...[]int) []int {
	if len(series) == 0 {
		return []int{}
	}
	out := make([]int, len(series[0]))
	for _, values := range series {
		for i, value := range values {
			if i < len(out) {
				out[i] += value
			}
		}
	}
	return out
}

func sumInts(values []int) int64 {
	var sum int64
	for _, value := range values {
		sum += int64(value)
	}
	return sum
}

func overviewDailyRows(labels []string, users []int, articles []int, topics []int, comments []int, reports []int, pending []int) []gin.H {
	rows := make([]gin.H, 0, len(labels))
	for i := len(labels) - 1; i >= 0; i-- {
		rows = append(rows, gin.H{
			"id":             len(labels) - i,
			"date":           labels[i],
			"newUsers":       valueAt(users, i),
			"newArticles":    valueAt(articles, i),
			"newTopics":      valueAt(topics, i),
			"newComments":    valueAt(comments, i),
			"reports":        valueAt(reports, i),
			"pendingReports": valueAt(pending, i),
		})
	}
	return rows
}

func valueAt(values []int, index int) int {
	if index < 0 || index >= len(values) {
		return 0
	}
	return values[index]
}

func overviewActivities(loginLogs []*adminpb.LoginLogInfo, operationLogs []*adminpb.OperationLogInfo) []gin.H {
	activities := make([]gin.H, 0, len(loginLogs)+len(operationLogs))
	for _, item := range operationLogs {
		activities = append(activities, gin.H{
			"type":      "operation",
			"summary":   fmt.Sprintf("%s %s", item.GetOperatorName(), item.GetTitle()),
			"detail":    item.GetBusinessType(),
			"timestamp": item.GetOperationTime(),
			"date":      formatOverviewTime(item.GetOperationTime()),
		})
	}
	for _, item := range loginLogs {
		activities = append(activities, gin.H{
			"type":      "login",
			"summary":   fmt.Sprintf("%s %s", item.GetUsername(), item.GetMessage()),
			"detail":    item.GetIp(),
			"timestamp": item.GetLoginTime(),
			"date":      formatOverviewTime(item.GetLoginTime()),
		})
	}
	sort.SliceStable(activities, func(i, j int) bool {
		return activities[i]["timestamp"].(int64) > activities[j]["timestamp"].(int64)
	})
	if len(activities) > 12 {
		return activities[:12]
	}
	return activities
}

func formatOverviewTime(timestamp int64) string {
	value := overviewTime(timestamp)
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04")
}

func formatInt(value int64) string {
	return fmt.Sprintf("%d", value)
}
