package utils

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
)

// MagicNumberMaxLength 定义最大魔数读取长度
// 7z 魔数 6 字节，rar 6 字节，ole 8 字节，8 字节足够覆盖全部已定义类型
const MagicNumberMaxLength = 8

// magicNumbers 定义常见文件类型的魔数
// P0 修复：补充 zip/rar/7z/docx/xlsx/pptx 等类型，覆盖 Office 文档与压缩包
// 注意：docx/xlsx/pptx 本质是 zip，魔数与 zip 相同（PK\x03\x04）
var magicNumbers = map[string][][]byte{
	"jpg":  {{0xFF, 0xD8, 0xFF}},
	"jpeg": {{0xFF, 0xD8, 0xFF}},
	"png":  {{0x89, 0x50, 0x4E, 0x47}},
	"gif":  {{0x47, 0x49, 0x46, 0x38}},
	"pdf":  {{0x25, 0x50, 0x44, 0x46}},
	// 压缩包
	"zip": {{0x50, 0x4B, 0x03, 0x04}, {0x50, 0x4B, 0x05, 0x06}, {0x50, 0x4B, 0x07, 0x08}},
	"rar": {{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07}, {0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x01, 0x00}},
	"7z":  {{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}},
	// Office Open XML（docx/xlsx/pptx 共享 zip 魔数）
	"docx": {{0x50, 0x4B, 0x03, 0x04}, {0x50, 0x4B, 0x05, 0x06}, {0x50, 0x4B, 0x07, 0x08}},
	"xlsx": {{0x50, 0x4B, 0x03, 0x04}, {0x50, 0x4B, 0x05, 0x06}, {0x50, 0x4B, 0x07, 0x08}},
	"pptx": {{0x50, 0x4B, 0x03, 0x04}, {0x50, 0x4B, 0x05, 0x06}, {0x50, 0x4B, 0x07, 0x08}},
}

// ValidateFileMagicNumber 根据魔数校验文件类型
// allowedTypes 允许的文件扩展名列表（不含点，如 []string{"jpg", "png", "pdf"}）
// 仅对 magicNumbers 中已定义的类型进行校验，未定义的类型（如 txt, doc）直接放行
func ValidateFileMagicNumber(fileHeader *multipart.FileHeader, allowedTypes []string) error {
	// 根据文件名推断扩展名，检查该类型是否有魔数定义
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != "" {
		ext = ext[1:] // 去掉点号，如 ".jpg" → "jpg"
	}
	if _, hasMagic := magicNumbers[ext]; !hasMagic {
		// 该文件类型没有魔数定义（如 txt, zip, docx），跳过校验
		return nil
	}

	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("无法打开文件: %w", err)
	}
	defer file.Close()

	buf := make([]byte, MagicNumberMaxLength)
	n, err := file.Read(buf)
	if err != nil {
		return fmt.Errorf("无法读取文件: %w", err)
	}
	buf = buf[:n]

	for _, t := range allowedTypes {
		t = strings.ToLower(t)
		patterns, ok := magicNumbers[t]
		if !ok {
			continue
		}
		for _, pattern := range patterns {
			if len(buf) >= len(pattern) && bytes.HasPrefix(buf, pattern) {
				return nil
			}
		}
	}

	return fmt.Errorf("文件魔数校验失败，类型不匹配")
}

// SanitizeFilename 清理文件名，防止路径遍历攻击
// 返回清理后的文件名，如果文件名包含非法路径元素则返回错误
func SanitizeFilename(filename string) (string, error) {
	if filename == "" {
		return "", fmt.Errorf("文件名为空")
	}

	base := filepath.Base(filename)
	if base == "." || base == "/" || base == "\\" {
		return "", fmt.Errorf("非法文件名")
	}

	if strings.Contains(base, "..") {
		return "", fmt.Errorf("文件名包含非法字符")
	}

	if strings.ContainsAny(base, "/\\") {
		return "", fmt.Errorf("文件名包含路径分隔符")
	}

	return base, nil
}
