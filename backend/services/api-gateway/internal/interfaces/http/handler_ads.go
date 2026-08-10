package http

import (
	stdhttp "net/http"
	"strconv"
	"time"

	"api-gateway/api/proto/adminpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type listAdminAdsRequest struct {
	Limit      int32     `json:"limit"`
	SinceID    jsonInt64 `json:"sinceId"`
	UntilID    jsonInt64 `json:"untilId"`
	Publishing *bool     `json:"publishing"`
}

type createAdminAdRequest struct {
	URL       string  `json:"url" binding:"required"`
	Memo      *string `json:"memo" binding:"required"`
	Place     string  `json:"place" binding:"required"`
	Priority  string  `json:"priority" binding:"required"`
	Ratio     *int32  `json:"ratio" binding:"required"`
	ExpiresAt *int64  `json:"expiresAt" binding:"required"`
	StartsAt  *int64  `json:"startsAt" binding:"required"`
	ImageURL  string  `json:"imageUrl" binding:"required"`
	DayOfWeek *int32  `json:"dayOfWeek" binding:"required"`
}

type updateAdminAdRequest struct {
	ID        *jsonInt64 `json:"id" binding:"required"`
	Memo      *string    `json:"memo"`
	URL       *string    `json:"url"`
	ImageURL  *string    `json:"imageUrl"`
	Place     *string    `json:"place"`
	Priority  *string    `json:"priority"`
	Ratio     *int32     `json:"ratio"`
	ExpiresAt *int64     `json:"expiresAt"`
	StartsAt  *int64     `json:"startsAt"`
	DayOfWeek *int32     `json:"dayOfWeek"`
}

type deleteAdminAdRequest struct {
	ID *jsonInt64 `json:"id" binding:"required"`
}

type adPayload struct {
	ID        string `json:"id"`
	ExpiresAt string `json:"expiresAt"`
	StartsAt  string `json:"startsAt"`
	Place     string `json:"place"`
	Priority  string `json:"priority"`
	Ratio     int32  `json:"ratio"`
	URL       string `json:"url"`
	ImageURL  string `json:"imageUrl"`
	Memo      string `json:"memo"`
	DayOfWeek int32  `json:"dayOfWeek"`
}

type publicAdPayload struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Place     string `json:"place"`
	Ratio     int32  `json:"ratio"`
	ImageURL  string `json:"imageUrl"`
	DayOfWeek int32  `json:"dayOfWeek"`
}

func (h *Handler) listAdminAds(c *gin.Context) {
	var req listAdminAdsRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.Limit < 1 || req.Limit > 100 || req.SinceID.Int64() < 0 || req.UntilID.Int64() < 0 {
		writeError(c, stdhttp.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.ListAds(ctx, &adminpb.ListAdsRequest{
		Actor:      currentActor(c),
		Limit:      req.Limit,
		SinceId:    req.SinceID.Int64(),
		UntilId:    req.UntilID.Int64(),
		Publishing: req.Publishing,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	items := make([]adPayload, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, adFromProto(item))
	}
	c.JSON(stdhttp.StatusOK, items)
}

func (h *Handler) createAdminAd(c *gin.Context) {
	var req createAdminAdRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Admin.CreateAd(ctx, &adminpb.CreateAdRequest{
		Actor:     currentActor(c),
		Url:       req.URL,
		Memo:      *req.Memo,
		Place:     req.Place,
		Priority:  req.Priority,
		Ratio:     *req.Ratio,
		ExpiresAt: *req.ExpiresAt,
		StartsAt:  *req.StartsAt,
		ImageUrl:  req.ImageURL,
		DayOfWeek: *req.DayOfWeek,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	if resp.GetAd() == nil {
		writeRPCError(c, status.Error(codes.Internal, "admin service returned an empty ad"))
		return
	}
	c.JSON(stdhttp.StatusOK, adFromProto(resp.GetAd()))
}

func (h *Handler) updateAdminAd(c *gin.Context) {
	var req updateAdminAdRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.ID.Int64() <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	_, err := h.clients.Admin.UpdateAd(ctx, &adminpb.UpdateAdRequest{
		Actor:     currentActor(c),
		Id:        req.ID.Int64(),
		Url:       req.URL,
		Memo:      req.Memo,
		Place:     req.Place,
		Priority:  req.Priority,
		Ratio:     req.Ratio,
		ExpiresAt: req.ExpiresAt,
		StartsAt:  req.StartsAt,
		ImageUrl:  req.ImageURL,
		DayOfWeek: req.DayOfWeek,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *Handler) deleteAdminAd(c *gin.Context) {
	var req deleteAdminAdRequest
	if !bindJSON(c, &req) {
		return
	}
	if req.ID.Int64() <= 0 {
		writeError(c, stdhttp.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	_, err := h.clients.Admin.DeleteAd(ctx, &adminpb.AdIDRequest{Actor: currentActor(c), Id: req.ID.Int64()})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func adFromProto(item *adminpb.AdInfo) adPayload {
	return adPayload{
		ID:        strconv.FormatInt(item.GetId(), 10),
		ExpiresAt: formatUnixMilli(item.GetExpiresAt()),
		StartsAt:  formatUnixMilli(item.GetStartsAt()),
		Place:     item.GetPlace(),
		Priority:  item.GetPriority(),
		Ratio:     item.GetRatio(),
		URL:       item.GetUrl(),
		ImageURL:  item.GetImageUrl(),
		Memo:      item.GetMemo(),
		DayOfWeek: item.GetDayOfWeek(),
	}
}

func publicAdFromProto(item *adminpb.ActiveAdInfo) publicAdPayload {
	return publicAdPayload{
		ID:        strconv.FormatInt(item.GetId(), 10),
		URL:       item.GetUrl(),
		Place:     item.GetPlace(),
		Ratio:     item.GetRatio(),
		ImageURL:  item.GetImageUrl(),
		DayOfWeek: item.GetDayOfWeek(),
	}
}

func formatUnixMilli(value int64) string {
	return time.UnixMilli(value).UTC().Format(time.RFC3339Nano)
}
