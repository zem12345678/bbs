package http

import (
	"encoding/xml"
	"net/http"
	"strconv"
	"strings"
	"time"

	"api-gateway/api/proto/contentpb"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const sitemapContentPageSize int32 = 100
const sitemapMaxOffset int64 = 1<<31 - 1

var publicSitemapStaticPaths = []string{
	"/",
	"/plaza",
	"/circles",
	"/help",
	"/resources",
	"/shop",
	"/member",
	"/more",
	"/topics",
	"/articles",
}

type sitemapIndexDocument struct {
	XMLName xml.Name           `xml:"sitemapindex"`
	XMLNS   string             `xml:"xmlns,attr"`
	Items   []sitemapIndexItem `xml:"sitemap"`
}

type sitemapIndexItem struct {
	Location string `xml:"loc"`
}

type sitemapURLSetDocument struct {
	XMLName xml.Name         `xml:"urlset"`
	XMLNS   string           `xml:"xmlns,attr"`
	Items   []sitemapURLItem `xml:"url"`
}

type sitemapURLItem struct {
	Location string `xml:"loc"`
	LastMod  string `xml:"lastmod,omitempty"`
}

func (h *Handler) robots(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("User-agent: *\nAllow: /\nDisallow: /api/\nDisallow: /chat\nDisallow: /room/\nDisallow: /dashboard/\nSitemap: "+h.publicURL(c, "/sitemap.xml")+"\n"))
}

func (h *Handler) sitemapIndex(c *gin.Context) {
	if !h.sitemapContentAvailable(c) {
		return
	}
	ctx, cancel := rpcContext(c)
	defer cancel()

	topics, err := h.clients.Content.ListTopics(ctx, &contentpb.ListTopicsRequest{
		Status: contentStatusPublished,
		Limit:  1,
		Sort:   "updated",
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}
	articles, err := h.clients.Content.ListArticles(ctx, &contentpb.ListArticlesRequest{
		Status: contentStatusPublished,
		Limit:  1,
		Sort:   "updated",
	})
	if err != nil {
		writeRPCError(c, err)
		return
	}

	items := []sitemapIndexItem{{Location: h.publicURL(c, "/sitemaps/static.xml")}}
	items = appendSitemapIndexItems(items, h.publicURL(c, ""), "topics", topics.GetTotal())
	items = appendSitemapIndexItems(items, h.publicURL(c, ""), "articles", articles.GetTotal())
	writeSitemapXML(c, sitemapIndexDocument{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		Items: items,
	})
}

func (h *Handler) sitemapPage(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "static.xml" {
		h.sitemapStaticPage(c)
		return
	}
	kind, page, ok := parseSitemapContentPage(name)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	if !h.sitemapContentAvailable(c) {
		return
	}
	offset, ok := sitemapPageOffset(page)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}

	ctx, cancel := rpcContext(c)
	defer cancel()
	baseURL := h.publicURL(c, "")
	switch kind {
	case "topics":
		resp, err := h.clients.Content.ListTopics(ctx, &contentpb.ListTopicsRequest{
			Status: contentStatusPublished,
			Limit:  sitemapContentPageSize,
			Offset: offset,
			Sort:   "updated",
		})
		if err != nil {
			writeRPCError(c, err)
			return
		}
		if page > sitemapPageCount(resp.GetTotal()) {
			c.Status(http.StatusNotFound)
			return
		}
		items := make([]sitemapURLItem, 0, len(resp.GetItems()))
		for _, topic := range resp.GetItems() {
			if topic.GetId() <= 0 || topic.GetStatus() != contentStatusPublished {
				continue
			}
			items = append(items, sitemapURLItem{
				Location: baseURL + "/topic/" + strconv.FormatInt(topic.GetId(), 10),
				LastMod:  sitemapLastModified(topic.GetUpdatedAt(), topic.GetPublishedAt(), topic.GetCreatedAt()),
			})
		}
		writeSitemapXML(c, sitemapURLSetDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", Items: items})
	case "articles":
		resp, err := h.clients.Content.ListArticles(ctx, &contentpb.ListArticlesRequest{
			Status: contentStatusPublished,
			Limit:  sitemapContentPageSize,
			Offset: offset,
			Sort:   "updated",
		})
		if err != nil {
			writeRPCError(c, err)
			return
		}
		if page > sitemapPageCount(resp.GetTotal()) {
			c.Status(http.StatusNotFound)
			return
		}
		items := make([]sitemapURLItem, 0, len(resp.GetItems()))
		for _, article := range resp.GetItems() {
			if article.GetId() <= 0 || article.GetStatus() != contentStatusPublished {
				continue
			}
			items = append(items, sitemapURLItem{
				Location: baseURL + "/article/" + strconv.FormatInt(article.GetId(), 10),
				LastMod:  sitemapLastModified(article.GetUpdatedAt(), article.GetPublishedAt(), article.GetCreatedAt()),
			})
		}
		writeSitemapXML(c, sitemapURLSetDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", Items: items})
	}
}

func (h *Handler) sitemapStaticPage(c *gin.Context) {
	baseURL := h.publicURL(c, "")
	items := make([]sitemapURLItem, 0, len(publicSitemapStaticPaths))
	for _, path := range publicSitemapStaticPaths {
		items = append(items, sitemapURLItem{Location: baseURL + path})
	}
	writeSitemapXML(c, sitemapURLSetDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", Items: items})
}

func (h *Handler) sitemapContentAvailable(c *gin.Context) bool {
	if h != nil && h.clients != nil && h.clients.Content != nil {
		return true
	}
	writeRPCError(c, status.Error(codes.Unavailable, "content service unavailable"))
	return false
}

func appendSitemapIndexItems(items []sitemapIndexItem, baseURL string, kind string, total int64) []sitemapIndexItem {
	for page := int64(1); page <= sitemapPageCount(total); page++ {
		items = append(items, sitemapIndexItem{
			Location: baseURL + "/sitemaps/" + kind + "-" + strconv.FormatInt(page, 10) + ".xml",
		})
	}
	return items
}

func parseSitemapContentPage(name string) (string, int64, bool) {
	base, ok := strings.CutSuffix(name, ".xml")
	if !ok {
		return "", 0, false
	}
	kind, pageText, ok := strings.Cut(base, "-")
	if !ok || (kind != "topics" && kind != "articles") {
		return "", 0, false
	}
	page, err := strconv.ParseInt(pageText, 10, 64)
	if err != nil || page <= 0 {
		return "", 0, false
	}
	return kind, page, true
}

func sitemapPageOffset(page int64) (int32, bool) {
	if page <= 0 || page-1 > sitemapMaxOffset/int64(sitemapContentPageSize) {
		return 0, false
	}
	return int32((page - 1) * int64(sitemapContentPageSize)), true
}

func sitemapPageCount(total int64) int64 {
	if total <= 0 {
		return 0
	}
	return 1 + (total-1)/int64(sitemapContentPageSize)
}

func sitemapLastModified(timestamps ...int64) string {
	for _, timestamp := range timestamps {
		if timestamp > 0 {
			return time.UnixMilli(timestamp).UTC().Format("2006-01-02")
		}
	}
	return ""
}

func writeSitemapXML(c *gin.Context, document any) {
	body, err := xml.Marshal(document)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "encode sitemap failed", "internal_error")
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "application/xml; charset=utf-8", append([]byte(xml.Header), body...))
}
