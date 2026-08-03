package http

import (
	"api-gateway/api/proto/contentpb"
	"api-gateway/pkg/http/response"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listTags(c *gin.Context) {
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.ListTags(ctx, &contentpb.ListTagsRequest{
		Limit: queryInt32(c, "limit", 12),
		Query: c.Query("q"),
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}

func (h *Handler) autocompleteTags(c *gin.Context) {
	var req autocompleteTagsRequest
	if !bindJSON(c, &req) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()
	resp, err := h.clients.Content.AutocompleteTags(ctx, &contentpb.AutocompleteTagsRequest{
		Query: req.Query,
		Limit: req.Limit,
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	response.Success(c, resp)
}
