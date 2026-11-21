package workspace

import (
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"
)

// ConvertConfluenceHTML 将 Confluence 导出的 HTML 转换为 Markdown
func ConvertConfluenceHTML(html string) (string, error) {
	// 简单的 HTML → Markdown 转换（处理 Confluence 常见标记）
	s := html

	// 移除 HTML 注释
	commentRe := regexp.MustCompile(`<!--[\s\S]*?-->`)
	s = commentRe.ReplaceAllString(s, "")

	// 标题 h1-h6
	for i := 6; i >= 1; i-- {
		re := regexp.MustCompile(fmt.Sprintf(`<h%d[^>]*>(.*?)</h%d>`, i, i))
		s = re.ReplaceAllStringFunc(s, func(m string) string {
			inner := re.FindStringSubmatch(m)[1]
			return strings.Repeat("#", i) + " " + stripTags(inner) + "\n\n"
		})
	}

	// 粗体/斜体
	boldRe := regexp.MustCompile(`<strong[^>]*>(.*?)</strong>`)
	s = boldRe.ReplaceAllString(s, "**$1**")
	italicRe := regexp.MustCompile(`<em[^>]*>(.*?)</em>`)
	s = italicRe.ReplaceAllString(s, "*$1*")

	// 代码块
	codeBlockRe := regexp.MustCompile(`<pre[^>]*><code[^>]*>(.*?)</code></pre>`)
	s = codeBlockRe.ReplaceAllString(s, "```\n$1\n```\n\n")

	// 行内代码
	codeRe := regexp.MustCompile(`<code[^>]*>(.*?)</code>`)
	s = codeRe.ReplaceAllString(s, "`$1`")

	// 链接
	linkRe := regexp.MustCompile(`<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	s = linkRe.ReplaceAllString(s, "[$2]($1)")

	// 图片
	imgRe := regexp.MustCompile(`<img[^>]*src="([^"]*)"[^>]*/?>`)
	s = imgRe.ReplaceAllString(s, "![]($1)")

	// 无序列表
	liRe := regexp.MustCompile(`<li[^>]*>(.*?)</li>`)
	s = liRe.ReplaceAllString(s, "- $1\n")
	ulRe := regexp.MustCompile(`<ul[^>]*>(.*?)</ul>`)
	s = ulRe.ReplaceAllString(s, "$1\n")

	// 有序列表
	olLiRe := regexp.MustCompile(`<ol[^>]*>(.*?)</ol>`)
	s = olLiRe.ReplaceAllStringFunc(s, func(m string) string {
		inner := olLiRe.FindStringSubmatch(m)[1]
		items := liRe.FindAllStringSubmatch(inner, -1)
		var result strings.Builder
		for i, item := range items {
			result.WriteString(fmt.Sprintf("%d. %s\n", i+1, item[1]))
		}
		return result.String() + "\n"
	})

	// 段落
	pRe := regexp.MustCompile(`<p[^>]*>(.*?)</p>`)
	s = pRe.ReplaceAllString(s, "$1\n\n")

	// 换行
	brRe := regexp.MustCompile(`<br\s*/?>`)
	s = brRe.ReplaceAllString(s, "\n")

	// 表格（简单处理：转换为 pipe table）
	tableRe := regexp.MustCompile(`<table[^>]*>([\s\S]*?)</table>`)
	s = tableRe.ReplaceAllStringFunc(s, func(m string) string {
		return convertHTMLTable(tableRe.FindStringSubmatch(m)[1])
	})

	// 移除剩余 HTML 标签
	s = stripTags(s)

	// 清理多余空行
	multiNewline := regexp.MustCompile(`\n{3,}`)
	s = multiNewline.ReplaceAllString(s, "\n\n")

	// HTML 实体解码
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")

	return strings.TrimSpace(s), nil
}

// ConvertNotionCSV 将 Notion 导出的 CSV 转换为 Markdown
func ConvertNotionCSV(csvContent string) (string, error) {
	reader := csv.NewReader(strings.NewReader(csvContent))
	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("parse CSV: %w", err)
	}
	if len(records) == 0 {
		return "", fmt.Errorf("empty CSV")
	}

	var sb strings.Builder
	// 标题行
	headers := records[0]
	sb.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	sb.WriteString("|" + strings.Repeat(" --- |", len(headers)) + "\n")

	// 数据行
	for _, row := range records[1:] {
		padded := make([]string, len(headers))
		for i, cell := range row {
			if i < len(row) {
				padded[i] = cell
			}
		}
		sb.WriteString("| " + strings.Join(padded, " | ") + " |\n")
	}

	return sb.String(), nil
}

func stripTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

func convertHTMLTable(html string) string {
	rowRe := regexp.MustCompile(`<tr[^>]*>([\s\S]*?)</tr>`)
	cellRe := regexp.MustCompile(`<t[dh][^>]*>([\s\S]*?)</t[dh]>`)

	rows := rowRe.FindAllStringSubmatch(html, -1)
	if len(rows) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, row := range rows {
		cells := cellRe.FindAllStringSubmatch(row[1], -1)
		var cellContents []string
		for _, c := range cells {
			cellContents = append(cellContents, strings.TrimSpace(stripTags(c[1])))
		}
		sb.WriteString("| " + strings.Join(cellContents, " | ") + " |\n")

		if i == 0 {
			sb.WriteString("|" + strings.Repeat(" --- |", len(cellContents)) + "\n")
		}
	}
	return sb.String() + "\n"
}
