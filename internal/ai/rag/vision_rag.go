package rag

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
)

// VisionRAGConfig 多模态视觉检索配置
type VisionRAGConfig struct {
	Enabled      bool    `yaml:"enabled" json:"enabled"`
	CaptionModel string  `yaml:"caption_model" json:"caption_model"`   // 图像描述模型
	MaxImageSize int     `yaml:"max_image_size" json:"max_image_size"` // 最大图像大小(bytes)
	SimilarityTh float64 `yaml:"similarity_threshold" json:"similarity_threshold"`
}

func DefaultVisionRAGConfig() VisionRAGConfig {
	return VisionRAGConfig{
		Enabled:      true,
		CaptionModel: "gpt-4-vision-preview",
		MaxImageSize: 5 * 1024 * 1024, // 5MB
		SimilarityTh: 0.6,
	}
}

// ImageContent 图像内容
type ImageContent struct {
	ID        int       `json:"id"`
	DocID     int       `json:"doc_id"`
	PageNum   int       `json:"page_num"`
	ImageData []byte    `json:"-"`          // 原始图像数据
	ImagePath string    `json:"image_path"` // 图像文件路径
	Caption   string    `json:"caption"`    // 生成的图像描述
	Tags      []string  `json:"tags"`       // 图像标签
	Embedding []float64 `json:"embedding,omitempty"`
}

// VisionRAG 多模态视觉检索
type VisionRAG struct {
	provider provider.Provider
	config   VisionRAGConfig
	images   map[int]*ImageContent // id -> image
}

func NewVisionRAG(p provider.Provider, cfg VisionRAGConfig) *VisionRAG {
	return &VisionRAG{
		provider: p,
		config:   cfg,
		images:   make(map[int]*ImageContent),
	}
}

// CaptionImage 为图像生成描述
func (v *VisionRAG) CaptionImage(ctx context.Context, image *ImageContent) (string, error) {
	if v.provider == nil {
		return "", fmt.Errorf("provider not available")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	prompt := `请详细描述这张图像的内容，包括：
1. 图像中有什么物体/场景
2. 关键的文字信息（如果有）
3. 图表/图形的含义（如果是图表）
4. 与技术文档相关的任何信息

描述应该简洁但完整，不超过200字。`

	resp, err := v.provider.Chat(ctx, "你是一个图像分析专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(256), provider.WithTemperature(0.3))
	if err != nil {
		return "", err
	}

	caption := strings.TrimSpace(resp.Content)
	image.Caption = caption

	return caption, nil
}

// CaptionBatch 批量为图像生成描述
func (v *VisionRAG) CaptionBatch(ctx context.Context, images []*ImageContent) error {
	for i, img := range images {
		caption, err := v.CaptionImage(ctx, img)
		if err != nil {
			logx.Infof("[VisionRAG] Failed to caption image %d: %v", i, err)
			continue
		}
		logx.Infof("[VisionRAG] Captioned image %d: %s", i, truncateForVision(caption, 50))
	}
	return nil
}

// SearchImages 搜索图像
func (v *VisionRAG) SearchImages(query string, limit int) []ImageContent {
	// 基于caption文本匹配搜索
	queryLower := strings.ToLower(query)
	var results []ImageContent

	for _, img := range v.images {
		if img.Caption == "" {
			continue
		}
		score := v.calculateTextSimilarity(queryLower, strings.ToLower(img.Caption))
		if score >= v.config.SimilarityTh {
			results = append(results, *img)
		}
	}

	// 截断到limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// calculateTextSimilarity 计算文本相似度（简单的关键词匹配）
func (v *VisionRAG) calculateTextSimilarity(query, caption string) float64 {
	queryWords := make(map[string]bool)
	for _, w := range strings.Fields(query) {
		if len(w) > 1 {
			queryWords[w] = true
		}
	}

	overlap := 0
	for _, w := range strings.Fields(caption) {
		if queryWords[w] {
			overlap++
		}
	}

	if len(queryWords) == 0 {
		return 0
	}
	return float64(overlap) / float64(len(queryWords))
}

// AddImage 添加图像到索引
func (v *VisionRAG) AddImage(img *ImageContent) {
	v.images[img.ID] = img
}

// GetImage 获取图像
func (v *VisionRAG) GetImage(id int) *ImageContent {
	return v.images[id]
}

// FormatCaptionAsMarkdown 将图像描述格式化为Markdown
func (v *VisionRAG) FormatCaptionAsMarkdown(img *ImageContent) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 图像 %d\n\n", img.ID))
	if img.Caption != "" {
		sb.WriteString(fmt.Sprintf("**描述:** %s\n\n", img.Caption))
	}
	if len(img.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("**标签:** %s\n\n", strings.Join(img.Tags, ", ")))
	}
	if img.ImagePath != "" {
		sb.WriteString(fmt.Sprintf("![图像](%s)\n", img.ImagePath))
	}
	return sb.String()
}

func truncateForVision(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
