package utils

import (
	"crypto/md5"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 会话持续时间（分钟），默认 60
var sessionDurationMin = 60

// SetSessionDuration 设置会话持续时间（分钟）
func SetSessionDuration(minutes int) {
	if minutes > 0 {
		sessionDurationMin = minutes
	}
}

// GetSessionDuration 获取会话持续时间（分钟）
func GetSessionDuration() int {
	return sessionDurationMin
}

// ConversationIDManager 会话ID管理器 (SOLID-SRP: 单一职责)
type ConversationIDManager struct {
	mu    sync.RWMutex      // 保护cache的并发访问
	cache map[string]string // 简单的内存缓存，生产环境可以使用Redis
}

// NewConversationIDManager 创建新的会话ID管理器
func NewConversationIDManager() *ConversationIDManager {
	return &ConversationIDManager{
		cache: make(map[string]string),
	}
}

// GenerateConversationID 生成会话ID
// 支持多种客户端 session 标识，优先级从高到低：
// 1. X-Conversation-ID (自定义会话 ID)
// 2. X-Session-ID / Session-ID (通用 session 头)
// 3. 基于 IP+UA+时间窗口 生成稳定会话 ID
func (c *ConversationIDManager) GenerateConversationID(ctx *gin.Context) string {
	// 记录所有可能的 session 相关头（用于调试）
	allHeaders := map[string]string{
		"X-Conversation-ID": ctx.GetHeader("X-Conversation-ID"),
		"X-Session-ID":      ctx.GetHeader("X-Session-ID"),
		"Session-ID":        ctx.GetHeader("Session-ID"),
		"X-Session-Id":      ctx.GetHeader("X-Session-Id"),
		"Session-Id":        ctx.GetHeader("Session-Id"),
		"X-Request-ID":      ctx.GetHeader("X-Request-ID"),
		"User-Agent":        ctx.GetHeader("User-Agent"),
	}

	// 临时日志：记录所有请求头（调试用）
	hasAnySession := false
	for k, v := range allHeaders {
		if v != "" && k != "User-Agent" {
			hasAnySession = true
			break
		}
	}
	if hasAnySession {
		fmt.Printf("[DEBUG] Session headers: %+v\n", allHeaders)
	}

	// 1. 优先使用客户端提供的会话 ID
	if customConvID := ctx.GetHeader("X-Conversation-ID"); customConvID != "" {
		return customConvID
	}

	// 2. 检查通用 session 头（Claude Code 可能使用）
	sessionID := ctx.GetHeader("X-Session-ID")
	if sessionID == "" {
		sessionID = ctx.GetHeader("Session-ID")
	}
	if sessionID == "" {
		sessionID = ctx.GetHeader("X-Session-Id")
	}
	if sessionID == "" {
		sessionID = ctx.GetHeader("Session-Id")
	}

	if sessionID != "" {
		// 有 session ID，基于它生成稳定的会话 ID
		clientIP := ctx.ClientIP()
		userAgent := ctx.GetHeader("User-Agent")
		windowMinutes := sessionDurationMin
		if windowMinutes <= 0 {
			windowMinutes = 60
		}
		windowStart := time.Now().Unix() / int64(windowMinutes*60)
		timeWindow := fmt.Sprintf("%d", windowStart)

		// 使用 session ID + IP + UA + 时间窗口生成稳定会话 ID
		clientSignature := fmt.Sprintf("%s|%s|%s|%s", sessionID, clientIP, userAgent, timeWindow)

		// 检查缓存
		c.mu.RLock()
		if cachedID, exists := c.cache[clientSignature]; exists {
			c.mu.RUnlock()
			fmt.Printf("[DEBUG] Using cached conversation ID: %s for session: %s\n", cachedID, sessionID)
			return cachedID
		}
		c.mu.RUnlock()

		// 生成基于特征的 MD5 哈希
		hash := md5.Sum([]byte(clientSignature))
		conversationID := fmt.Sprintf("conv-%x", hash[:8])

		// 缓存结果
		c.mu.Lock()
		c.cache[clientSignature] = conversationID
		c.mu.Unlock()

		fmt.Printf("[DEBUG] Generated new conversation ID: %s for session: %s\n", conversationID, sessionID)
		return conversationID
	}

	// 3. 没有任何 session 标识，生成随机 UUID
	randomID := fmt.Sprintf("conv-%s", GenerateUUID())
	fmt.Printf("[DEBUG] No session ID found, using random UUID: %s\n", randomID)
	return randomID
}

// GetOrCreateConversationID 获取或创建会话ID
func (c *ConversationIDManager) GetOrCreateConversationID(ctx *gin.Context) string {
	return c.GenerateConversationID(ctx)
}

// InvalidateOldSessions 清理过期的会话缓存
// SOLID-SRP: 单独的清理职责，避免内存泄漏
func (c *ConversationIDManager) InvalidateOldSessions() {
	// 简单实现：清空所有缓存，依赖时间窗口重新生成
	// 生产环境可以实现基于TTL的精确清理
	c.mu.Lock()
	c.cache = make(map[string]string)
	c.mu.Unlock()
}

// 全局实例 - 单例模式 (SOLID-DIP: 提供抽象访问)
var globalConversationIDManager = NewConversationIDManager()

// GenerateStableConversationID 生成稳定的会话ID的全局函数
// 为了向后兼容和简化调用，提供全局访问函数
func GenerateStableConversationID(ctx *gin.Context) string {
	return globalConversationIDManager.GetOrCreateConversationID(ctx)
}

// GenerateStableAgentContinuationID 生成稳定的代理延续GUID
// 基于客户端特征生成确定性的标准GUID格式，遵循SOLID-SRP原则
func GenerateStableAgentContinuationID(ctx *gin.Context) string {
	// 向后兼容：如果没有提供context，使用随机UUID
	if ctx == nil {
		return GenerateUUID()
	}

	// 检查是否有自定义的代理延续ID头（优先级最高）
	if customAgentID := ctx.GetHeader("X-Agent-Continuation-ID"); customAgentID != "" {
		return customAgentID
	}

	// 提取客户端特征信息
	clientSignature := buildAgentClientSignature(ctx)

	// 生成确定性GUID
	return generateDeterministicGUID(clientSignature, "agent")
}

// buildAgentClientSignature 构建代理客户端特征签名 (SOLID-SRP: 单一职责)
func buildAgentClientSignature(ctx *gin.Context) string {
	clientIP := ctx.ClientIP()
	userAgent := ctx.GetHeader("User-Agent")

	// 统一使用1小时时间窗口，与ConversationId保持一致
	// 确保在同一会话内AgentContinuationId保持稳定
	timeWindow := time.Now().Format("2006010215") // 精确到小时

	return fmt.Sprintf("agent|%s|%s|%s", clientIP, userAgent, timeWindow)
}

// generateDeterministicGUID 基于输入字符串生成确定性GUID (SOLID-SRP: 单一职责)
// 遵循UUID v5规范，使用MD5哈希生成标准GUID格式
func generateDeterministicGUID(input, namespace string) string {
	// 在输入中加入命名空间以避免冲突
	namespacedInput := fmt.Sprintf("%s|%s", namespace, input)

	// 生成MD5哈希
	hash := md5.Sum([]byte(namespacedInput))

	// 按照UUID格式重新排列字节
	// 设置版本位 (Version 5 - 基于命名空间的UUID)
	hash[6] = (hash[6] & 0x0f) | 0x50 // Version 5
	hash[8] = (hash[8] & 0x3f) | 0x80 // Variant bits

	// 格式化为标准GUID格式: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
}

// ExtractClientInfo 提取客户端信息用于调试和日志
func ExtractClientInfo(ctx *gin.Context) map[string]string {
	return map[string]string{
		"client_ip":            ctx.ClientIP(),
		"user_agent":           ctx.GetHeader("User-Agent"),
		"custom_conv_id":       ctx.GetHeader("X-Conversation-ID"),
		"custom_agent_cont_id": ctx.GetHeader("X-Agent-Continuation-ID"),
		"forwarded_for":        ctx.GetHeader("X-Forwarded-For"),
	}
}
