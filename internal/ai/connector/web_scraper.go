package connector

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/listen808/citebase/internal/brand"
	"github.com/listen808/citebase/internal/utils/netguard"
)

// ScrapedContent holds the result of scraping a single page.
type ScrapedContent struct {
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Content   string    `json:"content"` // Markdown format
	Links     []string  `json:"links"`
	CrawlTime time.Time `json:"crawl_time"`
}

// WebScraper crawls external URLs and extracts content as Markdown.
type WebScraper struct {
	httpClient *http.Client
	maxDepth   int
	maxPages   int
	maxBodyMB  int
}

// NewWebScraper creates a WebScraper with sensible defaults.
func NewWebScraper() *WebScraper {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &WebScraper{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			// SSRF 防护的三层，缺一层都还能打内网：
			//   1) scheme 与主机名形式的本地地址（CheckURL / IsBlockedHost，请求前）；
			//   2) 连接期解析出的**每个** IP 都要判定，并且连的就是刚判过的那个 IP
			//      （netguard.DialContext）—— 少了这层，短 TTL 的 DNS 记录就能让
			//      "校验一次、连接再一次"的两回解析给出两个答案；
			//   3) 每一跳 3xx 重定向复查目标（Go 默认跟 10 跳，公网站点 302 到
			//      169.254.169.254/127.0.0.1 是最省事的一条内网路线）。
			Transport: &http.Transport{DialContext: netguard.DialContext(dialer)},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if err := IsBlockedHost(req.URL.String()); err != nil {
					return fmt.Errorf("redirect target blocked: %w", err)
				}
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		maxDepth:  2,
		maxPages:  50,
		maxBodyMB: 1,
	}
}

// ScrapeURL fetches a single URL and returns its content in Markdown.
func (s *WebScraper) ScrapeURL(ctx context.Context, rawURL string) (*ScrapedContent, error) {
	doc, finalURL, err := s.fetchPage(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return s.extractContent(doc, finalURL), nil
}

// ScrapeSite crawls an entire site starting from startURL, following same-domain links.
func (s *WebScraper) ScrapeSite(ctx context.Context, startURL string, maxPages int) ([]*ScrapedContent, error) {
	if maxPages <= 0 {
		maxPages = s.maxPages
	}
	if maxPages > s.maxPages {
		maxPages = s.maxPages
	}

	base, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("invalid start URL: %w", err)
	}

	var results []*ScrapedContent
	visited := make(map[string]bool)

	type crawlTask struct {
		url   string
		depth int
	}

	queue := []crawlTask{{url: startURL, depth: 0}}

	for len(queue) > 0 && len(results) < maxPages {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		task := queue[0]
		queue = queue[1:]

		if visited[task.url] {
			continue
		}
		visited[task.url] = true

		doc, finalURL, err := s.fetchPage(ctx, task.url)
		if err != nil {
			continue
		}
		content := s.extractContent(doc, finalURL)
		results = append(results, content)

		// Enqueue same-domain links if within depth
		if task.depth < s.maxDepth && len(results) < maxPages {
			for _, link := range content.Links {
				if visited[link] {
					continue
				}
				linkURL, err := url.Parse(link)
				if err != nil {
					continue
				}
				if linkURL.Host == base.Host {
					queue = append(queue, crawlTask{url: link, depth: task.depth + 1})
				}
			}
		}
	}

	return results, nil
}

// fetchPage retrieves a URL and parses its HTML body.
// 在发起 HTTP 请求前调用 IsBlockedHost 校验，防止 SSRF 攻击
// （含 ScrapeSite 跟随的子链接、corrective_rag 调用方）
func (s *WebScraper) fetchPage(ctx context.Context, rawURL string) (*html.Node, string, error) {
	if err := IsBlockedHost(rawURL); err != nil {
		return nil, "", fmt.Errorf("SSRF blocked: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", brand.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, rawURL)
	}

	maxBytes := int64(s.maxBodyMB) * 1024 * 1024
	limited := io.LimitReader(resp.Body, maxBytes)

	doc, err := html.Parse(limited)
	if err != nil {
		return nil, "", fmt.Errorf("parse HTML: %w", err)
	}

	return doc, resp.Request.URL.String(), nil
}

// extractContent walks the parsed HTML and produces a ScrapedContent.
func (s *WebScraper) extractContent(doc *html.Node, pageURL string) *ScrapedContent {
	title := extractTitle(doc)
	links := extractLinks(doc, pageURL)

	// Prefer <article> or <main>, fall back to <body>
	contentNode := findFirst(doc, atom.Article)
	if contentNode == nil {
		contentNode = findFirst(doc, atom.Main)
	}
	if contentNode == nil {
		contentNode = findFirst(doc, atom.Body)
	}
	if contentNode == nil {
		contentNode = doc
	}

	md := htmlToMarkdown(contentNode)

	return &ScrapedContent{
		URL:       pageURL,
		Title:     title,
		Content:   md,
		Links:     links,
		CrawlTime: time.Now(),
	}
}

// extractTitle finds the <title> text.
func extractTitle(n *html.Node) string {
	title := findFirst(n, atom.Title)
	if title == nil {
		return ""
	}
	return textContent(title)
}

// extractLinks collects all <a href> absolute URLs.
func extractLinks(n *html.Node, baseURL string) []string {
	var links []string
	base, _ := url.Parse(baseURL)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.DataAtom == atom.A {
			for _, attr := range node.Attr {
				if attr.Key == "href" {
					ref := strings.TrimSpace(attr.Val)
					if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "javascript:") {
						continue
					}
					abs := resolveURL(base, ref)
					if abs != "" {
						links = append(links, abs)
					}
				}
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return links
}

func resolveURL(base *url.URL, ref string) string {
	if base == nil {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return base.ResolveReference(u).String()
}

// findFirst returns the first element matching the given atom.
func findFirst(n *html.Node, a atom.Atom) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirst(c, a); found != nil {
			return found
		}
	}
	return nil
}

// textContent returns all text under a node, concatenated.
func textContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(sb.String())
}

// htmlToMarkdown converts an HTML node tree to a simple Markdown string.
func htmlToMarkdown(n *html.Node) string {
	var sb strings.Builder
	convertNode(&sb, n, 0)
	// Clean up excessive blank lines
	result := sb.String()
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(result)
}

func convertNode(sb *strings.Builder, n *html.Node, depth int) {
	if n == nil {
		return
	}

	switch n.Type {
	case html.TextNode:
		text := strings.TrimSpace(n.Data)
		if text != "" {
			sb.WriteString(text)
		}
		return
	case html.ElementNode:
		// skip script, style, nav, footer, header
		switch n.DataAtom {
		case atom.Script, atom.Style, atom.Nav, atom.Footer:
			return
		}

		switch n.DataAtom {
		case atom.H1:
			sb.WriteString("\n\n# ")
			writeChildrenText(sb, n)
			sb.WriteString("\n")
			return
		case atom.H2:
			sb.WriteString("\n\n## ")
			writeChildrenText(sb, n)
			sb.WriteString("\n")
			return
		case atom.H3:
			sb.WriteString("\n\n### ")
			writeChildrenText(sb, n)
			sb.WriteString("\n")
			return
		case atom.H4:
			sb.WriteString("\n\n#### ")
			writeChildrenText(sb, n)
			sb.WriteString("\n")
			return
		case atom.H5:
			sb.WriteString("\n\n##### ")
			writeChildrenText(sb, n)
			sb.WriteString("\n")
			return
		case atom.H6:
			sb.WriteString("\n\n###### ")
			writeChildrenText(sb, n)
			sb.WriteString("\n")
			return
		case atom.P:
			sb.WriteString("\n\n")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				convertNode(sb, c, depth)
			}
			sb.WriteString("\n")
			return
		case atom.Br:
			sb.WriteString("\n")
			return
		case atom.Li:
			sb.WriteString("\n- ")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				convertNode(sb, c, depth)
			}
			return
		case atom.Ul, atom.Ol:
			sb.WriteString("\n")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				convertNode(sb, c, depth+1)
			}
			sb.WriteString("\n")
			return
		case atom.A:
			href := getAttr(n, "href")
			text := textContent(n)
			if text != "" && href != "" && !strings.HasPrefix(href, "javascript:") {
				sb.WriteString("[")
				sb.WriteString(text)
				sb.WriteString("](")
				sb.WriteString(href)
				sb.WriteString(")")
			} else if text != "" {
				sb.WriteString(text)
			}
			return
		case atom.Strong, atom.B:
			sb.WriteString("**")
			writeChildrenText(sb, n)
			sb.WriteString("**")
			return
		case atom.Em, atom.I:
			sb.WriteString("*")
			writeChildrenText(sb, n)
			sb.WriteString("*")
			return
		case atom.Code:
			sb.WriteString("`")
			writeChildrenText(sb, n)
			sb.WriteString("`")
			return
		case atom.Blockquote:
			sb.WriteString("\n\n> ")
			writeChildrenText(sb, n)
			sb.WriteString("\n")
			return
		case atom.Img:
			alt := getAttr(n, "alt")
			src := getAttr(n, "src")
			if src != "" {
				sb.WriteString("![")
				sb.WriteString(alt)
				sb.WriteString("](")
				sb.WriteString(src)
				sb.WriteString(")")
			}
			return
		case atom.Pre:
			sb.WriteString("\n\n```\n")
			sb.WriteString(textContent(n))
			sb.WriteString("\n```\n")
			return
		case atom.Table:
			convertTable(sb, n)
			return
		case atom.Tr:
			// handled inside convertTable
			return
		case atom.Td, atom.Th:
			// handled inside convertTable
			return
		case atom.Thead, atom.Tbody, atom.Tfoot:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				convertNode(sb, c, depth)
			}
			return
		}

		// Default: recurse into children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			convertNode(sb, c, depth)
		}
	}
}

// writeChildrenText writes only the text content of all children.
func writeChildrenText(sb *strings.Builder, n *html.Node) {
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		} else if node.Type == html.ElementNode {
			// Handle inline elements
			switch node.DataAtom {
			case atom.Strong, atom.B:
				sb.WriteString("**")
				for c := node.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				sb.WriteString("**")
				return
			case atom.Em, atom.I:
				sb.WriteString("*")
				for c := node.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				sb.WriteString("*")
				return
			case atom.Code:
				sb.WriteString("`")
				for c := node.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
				sb.WriteString("`")
				return
			case atom.A:
				href := getAttr(node, "href")
				text := textContent(node)
				if text != "" && href != "" {
					sb.WriteString("[")
					sb.WriteString(text)
					sb.WriteString("](")
					sb.WriteString(href)
					sb.WriteString(")")
				} else {
					sb.WriteString(text)
				}
				return
			case atom.Br:
				sb.WriteString("\n")
				return
			}
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(n)
}

// convertTable converts an HTML <table> to a Markdown table.
func convertTable(sb *strings.Builder, n *html.Node) {
	var rows [][]string
	var collectRows func(*html.Node)
	collectRows = func(node *html.Node) {
		if node.Type == html.ElementNode && node.DataAtom == atom.Tr {
			var cells []string
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.DataAtom == atom.Td || c.DataAtom == atom.Th) {
					cells = append(cells, strings.TrimSpace(textContent(c)))
				}
			}
			if len(cells) > 0 {
				rows = append(rows, cells)
			}
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			collectRows(c)
		}
	}
	collectRows(n)

	if len(rows) == 0 {
		return
	}

	// Normalize column count
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}
	for i := range rows {
		for len(rows[i]) < maxCols {
			rows[i] = append(rows[i], "")
		}
	}

	sb.WriteString("\n\n")
	// Header row
	sb.WriteString("| " + strings.Join(rows[0], " | ") + " |")
	sb.WriteString("\n")
	// Separator
	sb.WriteString("|")
	for i := 0; i < maxCols; i++ {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")
	// Data rows
	for _, row := range rows[1:] {
		sb.WriteString("| " + strings.Join(row, " | ") + " |")
		sb.WriteString("\n")
	}
}

// getAttr returns the value of an attribute on an element node.
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
