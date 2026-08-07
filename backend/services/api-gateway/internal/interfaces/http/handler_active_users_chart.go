package http

import (
	"context"
	"net/http"
	"sync"

	"api-gateway/api/proto/commentpb"
	"api-gateway/api/proto/contentpb"
	"api-gateway/api/proto/userpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type activeUsersChartResult struct {
	ReadWrite              []int64
	Read                   []int64
	Write                  []int64
	RegisteredWithinWeek   []int64
	RegisteredWithinMonth  []int64
	RegisteredWithinYear   []int64
	RegisteredOutsideWeek  []int64
	RegisteredOutsideMonth []int64
	RegisteredOutsideYear  []int64
}

func (h *Handler) activeUsersChart(c *gin.Context) {
	if h == nil || h.clients == nil || h.clients.UserActiveUsersCharts == nil || h.clients.Content == nil || h.clients.Comment == nil {
		writeRPCError(c, status.Error(codes.Unavailable, "active users chart service unavailable"))
		return
	}
	params, ok := bindNoteChartRequest(c, false)
	if !ok {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	result, err := h.getActiveUsersChart(ctx, params)
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"readWrite":              nonNilInt64s(result.ReadWrite),
		"read":                   nonNilInt64s(result.Read),
		"write":                  nonNilInt64s(result.Write),
		"registeredWithinWeek":   nonNilInt64s(result.RegisteredWithinWeek),
		"registeredWithinMonth":  nonNilInt64s(result.RegisteredWithinMonth),
		"registeredWithinYear":   nonNilInt64s(result.RegisteredWithinYear),
		"registeredOutsideWeek":  nonNilInt64s(result.RegisteredOutsideWeek),
		"registeredOutsideMonth": nonNilInt64s(result.RegisteredOutsideMonth),
		"registeredOutsideYear":  nonNilInt64s(result.RegisteredOutsideYear),
	})
}

func (h *Handler) getActiveUsersChart(ctx context.Context, params noteChartParams) (activeUsersChartResult, error) {
	userRequest := &userpb.ActiveUsersChartRequest{Span: params.Span, Limit: params.Limit, Offset: params.Offset}
	contentRequest := &contentpb.ActiveUsersChartRequest{Span: params.Span, Limit: params.Limit, Offset: params.Offset}
	commentRequest := &commentpb.ActiveUsersChartRequest{Span: params.Span, Limit: params.Limit, Offset: params.Offset}

	var userResult *userpb.ActiveUsersChartResponse
	var contentResult *contentpb.ActiveUsersChartResponse
	var commentResult *commentpb.ActiveUsersChartResponse
	var userErr, contentErr, commentErr error
	var calls sync.WaitGroup
	calls.Add(3)
	go func() {
		defer calls.Done()
		userResult, userErr = h.clients.UserActiveUsersCharts.GetActiveUsersChart(ctx, userRequest)
	}()
	go func() {
		defer calls.Done()
		contentResult, contentErr = h.clients.Content.GetActiveUsersChart(ctx, contentRequest)
	}()
	go func() {
		defer calls.Done()
		commentResult, commentErr = h.clients.Comment.GetActiveUsersChart(ctx, commentRequest)
	}()
	calls.Wait()
	if userErr != nil {
		return activeUsersChartResult{}, userErr
	}
	if contentErr != nil {
		return activeUsersChartResult{}, contentErr
	}
	if commentErr != nil {
		return activeUsersChartResult{}, commentErr
	}

	limit := int(params.Limit)
	if len(userResult.GetBuckets()) != limit || len(contentResult.GetBuckets()) != limit || len(commentResult.GetBuckets()) != limit {
		return activeUsersChartResult{}, status.Error(codes.DataLoss, "invalid active users chart response")
	}
	result := newActiveUsersChartResult(limit)
	for index := 0; index < limit; index++ {
		userBucket := userResult.GetBuckets()[index]
		readers := userIDSet(userBucket.GetReadUserIds())
		writers := userIDSet(contentResult.GetBuckets()[index].GetWriterUserIds(), commentResult.GetBuckets()[index].GetWriterUserIds())
		result.Read[index] = int64(len(readers))
		result.Write[index] = int64(len(writers))
		for userID := range readers {
			if _, ok := writers[userID]; ok {
				result.ReadWrite[index]++
			}
		}
		result.RegisteredWithinWeek[index] = userBucket.GetRegisteredWithinWeek()
		result.RegisteredWithinMonth[index] = userBucket.GetRegisteredWithinMonth()
		result.RegisteredWithinYear[index] = userBucket.GetRegisteredWithinYear()
		result.RegisteredOutsideWeek[index] = userBucket.GetRegisteredOutsideWeek()
		result.RegisteredOutsideMonth[index] = userBucket.GetRegisteredOutsideMonth()
		result.RegisteredOutsideYear[index] = userBucket.GetRegisteredOutsideYear()
	}
	return result, nil
}

func newActiveUsersChartResult(length int) activeUsersChartResult {
	return activeUsersChartResult{
		ReadWrite: make([]int64, length), Read: make([]int64, length), Write: make([]int64, length),
		RegisteredWithinWeek: make([]int64, length), RegisteredWithinMonth: make([]int64, length),
		RegisteredWithinYear: make([]int64, length), RegisteredOutsideWeek: make([]int64, length),
		RegisteredOutsideMonth: make([]int64, length), RegisteredOutsideYear: make([]int64, length),
	}
}

func userIDSet(groups ...[]int64) map[int64]struct{} {
	result := make(map[int64]struct{})
	for _, group := range groups {
		for _, userID := range group {
			result[userID] = struct{}{}
		}
	}
	return result
}
