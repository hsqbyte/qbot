package services

import (
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/hsqbyte/qbot/src/core"
	"github.com/sashabaranov/go-openai"
)

// CQ:image 格式示例:
// [CQ:image,file=xxx.image,subType=0,url=https://gchat.qpic.cn/xxx,file_size=123]
// URL 可能包含逗号之前的各种参数，需要精确匹配 url= 到下一个已知 key 或 ] 结束
var cqImageRegex = regexp.MustCompile(`\[CQ:image[^\]]*\]`)
var cqImageURLRegex = regexp.MustCompile(`url=(https?://[^\],\]]+)`)

// ImageInfo 从消息中提取的图片信息
type ImageInfo struct {
	URL string
}

// ExtractImages 从 CQ 码消息中提取所有图片 URL
func ExtractImages(rawMessage string) []ImageInfo {
	// 先找到所有 CQ:image 块
	blocks := cqImageRegex.FindAllString(rawMessage, -1)
	var images []ImageInfo

	for _, block := range blocks {
		// 从每个块中提取 url=
		match := cqImageURLRegex.FindStringSubmatch(block)
		if len(match) > 1 {
			url := match[1]
			core.Log.Debugf("📷 提取图片 URL: %s", url)
			images = append(images, ImageInfo{URL: url})
		}
	}
	return images
}

// StripImageCQ 去除消息中的图片 CQ 码，只保留文本
func StripImageCQ(rawMessage string) string {
	return strings.TrimSpace(cqImageRegex.ReplaceAllString(rawMessage, ""))
}

// DownloadImageAsBase64 下载图片并转为 base64 data URI
// 腾讯 CDN (gchat.qpic.cn) 需要特定请求头
func DownloadImageAsBase64(imageURL string) (string, error) {
	// NapCat CQ码中的URL可能包含HTML实体编码（&amp; → &）
	imageURL = html.UnescapeString(imageURL)

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 腾讯 CDN 需要这些请求头，否则返回 400
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://qq.com")
	req.Header.Set("Accept", "image/*,*/*")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("图片请求返回 %d (URL: %s)", resp.StatusCode, imageURL)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取图片数据失败: %w", err)
	}

	// 限制图片大小（防止 base64 过大导致 API 超限）
	const maxImageSize = 10 * 1024 * 1024 // 10MB
	if len(data) > maxImageSize {
		return "", fmt.Errorf("图片过大: %d bytes (最大 %d)", len(data), maxImageSize)
	}

	// 检测 content-type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" || !strings.HasPrefix(contentType, "image/") {
		contentType = http.DetectContentType(data)
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	dataURI := fmt.Sprintf("data:%s;base64,%s", contentType, b64)

	core.Log.Infof("📷 下载图片成功: %d bytes, type=%s", len(data), contentType)
	return dataURI, nil
}

// BuildMultimodalContent 构建多模态消息内容（文字+图片）
func BuildMultimodalContent(text string, imageURLs []string) []openai.ChatMessagePart {
	var parts []openai.ChatMessagePart

	if text != "" {
		parts = append(parts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeText,
			Text: text,
		})
	}

	for _, url := range imageURLs {
		parts = append(parts, openai.ChatMessagePart{
			Type: openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{
				URL:    url,
				Detail: openai.ImageURLDetailAuto,
			},
		})
	}

	return parts
}
