package utils

import "fmt"

// FormatGroupMsg 格式化群消息显示
func FormatGroupMsg(groupID int64, nickname, message string) string {
	return fmt.Sprintf("[群%d] %s: %s", groupID, nickname, message)
}

// FormatPrivateMsg 格式化私聊消息显示
func FormatPrivateMsg(nickname, message string) string {
	return fmt.Sprintf("[私聊] %s: %s", nickname, message)
}

// ContainsAny 检查字符串是否包含任意一个关键词
func ContainsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if len(kw) > 0 && contains(s, kw) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
