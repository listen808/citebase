package ai

import (
	"errors"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai/connector"
	"github.com/listen808/citebase/internal/response"
)

// ConnectorAPI handles external data source connector endpoints.
type ConnectorAPI struct {
	scraper *connector.WebScraper
	parser  *connector.FileParser
}

// NewConnectorAPI creates a new ConnectorAPI.
func NewConnectorAPI() *ConnectorAPI {
	return &ConnectorAPI{
		scraper: connector.NewWebScraper(),
		parser:  connector.NewFileParser(),
	}
}

// ScrapeURL scrapes a single URL and returns its Markdown content.
func (api *ConnectorAPI) ScrapeURL(c *gin.Context) {
	var req struct {
		URL     string `json:"url" binding:"required"`
		SpaceID uint64 `json:"space_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgURLRequired)
		return
	}

	if err := connector.IsBlockedHost(req.URL); err != nil {
		logx.Infof("[connector] ScrapeURL SSRF blocked: url=%s, err=%v", req.URL, err)
		response.BadRequest(c, fmt.Sprintf("URL 校验失败: %v", err))
		return
	}

	content, err := api.scraper.ScrapeURL(c.Request.Context(), req.URL)
	if err != nil {
		logx.Infof("[connector] ScrapeURL error: url=%s, err=%v", req.URL, err)
		response.Error(c, http.StatusInternalServerError, 500, fmt.Sprintf("爬取失败: %v", err))
		return
	}

	response.Success(c, gin.H{
		"url":        content.URL,
		"title":      content.Title,
		"content":    content.Content,
		"word_count": len([]rune(content.Content)),
		"links":      content.Links,
		"crawl_time": content.CrawlTime,
		"space_id":   req.SpaceID,
	})
}

// ScrapeSite crawls an entire site starting from a URL.
func (api *ConnectorAPI) ScrapeSite(c *gin.Context) {
	var req struct {
		URL      string `json:"url" binding:"required"`
		SpaceID  uint64 `json:"space_id"`
		MaxPages int    `json:"max_pages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgURLRequired)
		return
	}

	if err := connector.IsBlockedHost(req.URL); err != nil {
		logx.Infof("[connector] ScrapeSite SSRF blocked: url=%s, err=%v", req.URL, err)
		response.BadRequest(c, fmt.Sprintf("URL 校验失败: %v", err))
		return
	}

	if req.MaxPages <= 0 {
		req.MaxPages = 10
	}
	if req.MaxPages > 50 {
		req.MaxPages = 50
	}

	contents, err := api.scraper.ScrapeSite(c.Request.Context(), req.URL, req.MaxPages)
	if err != nil {
		logx.Infof("[connector] ScrapeSite error: url=%s, err=%v", req.URL, err)
		response.Error(c, http.StatusInternalServerError, 500, fmt.Sprintf("爬取站点失败: %v", err))
		return
	}

	// Build summary
	type pageSummary struct {
		URL       string `json:"url"`
		Title     string `json:"title"`
		WordCount int    `json:"word_count"`
		Content   string `json:"content"`
	}

	var pages []pageSummary
	for _, c := range contents {
		pages = append(pages, pageSummary{
			URL:       c.URL,
			Title:     c.Title,
			WordCount: len([]rune(c.Content)),
			Content:   c.Content,
		})
	}

	response.Success(c, gin.H{
		"pages":    pages,
		"total":    len(pages),
		"space_id": req.SpaceID,
	})
}

// UploadFile handles file upload and parsing.
func (api *ConnectorAPI) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequestKey(c, response.MsgNoFileChosen)
		return
	}
	defer file.Close()

	// Limit file size to 10MB
	if header.Size > 10*1024*1024 {
		response.BadRequestKey(c, response.MsgFileTooLarge)
		return
	}

	filename := header.Filename
	if !connector.IsSupported(filename) {
		supported := strings.Join(connector.SupportedExtensions, ", ")
		response.BadRequest(c, fmt.Sprintf("不支持的文件类型，支持: %s", supported))
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "读取文件失败")
		return
	}

	content, err := api.parser.ParseFile(filename, data)
	if err != nil {
		// P1-4：扫描件 PDF 给出可行动的提示，而不是笼统的"解析文件失败"
		if errors.Is(err, connector.ErrPDFNoTextLayer) {
			logx.Warnf("[connector] no text layer: filename=%s", filename)
			response.Error(c, http.StatusUnprocessableEntity, 422,
				"该文件没有可提取的文本层（疑似扫描件或图片型 PDF），请先做 OCR 再导入")
			return
		}
		logx.Infof("[connector] ParseFile error: filename=%s, err=%v", filename, err)
		response.Error(c, http.StatusInternalServerError, 500, fmt.Sprintf("解析文件失败: %v", err))
		return
	}

	spaceIDStr := c.PostForm("space_id")
	var spaceID uint64
	fmt.Sscanf(spaceIDStr, "%d", &spaceID)

	response.Success(c, gin.H{
		"filename":   filename,
		"content":    content,
		"word_count": len([]rune(content)),
		"space_id":   spaceID,
	})
}
