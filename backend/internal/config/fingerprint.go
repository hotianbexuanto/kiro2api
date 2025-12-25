package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// FingerprintManager 管理动态指纹生成
type FingerprintManager struct {
	mu           sync.RWMutex
	fingerprints map[string]string // tokenID -> fingerprint
	baseTime     int64             // 基准时间戳
}

var (
	globalFPManager *FingerprintManager
	fpOnce          sync.Once
)

// GetFingerprintManager 获取全局指纹管理器
func GetFingerprintManager() *FingerprintManager {
	fpOnce.Do(func() {
		globalFPManager = &FingerprintManager{
			fingerprints: make(map[string]string),
			baseTime:     time.Now().UnixNano(),
		}
	})
	return globalFPManager
}

// GenerateFingerprint 为指定 tokenID 生成或获取指纹
// tokenID 可以是 refreshToken 的前缀或其他唯一标识
func (m *FingerprintManager) GenerateFingerprint(tokenID string) string {
	m.mu.RLock()
	if fp, ok := m.fingerprints[tokenID]; ok {
		m.mu.RUnlock()
		return fp
	}
	m.mu.RUnlock()

	// 生成新指纹
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if fp, ok := m.fingerprints[tokenID]; ok {
		return fp
	}

	// 使用 tokenID + 基准时间 + 随机因子生成指纹
	data := fmt.Sprintf("%s-%d-%d", tokenID, m.baseTime, len(m.fingerprints))
	hash := sha256.Sum256([]byte(data))
	fp := hex.EncodeToString(hash[:])

	m.fingerprints[tokenID] = fp
	return fp
}

// GetFingerprint 获取已存在的指纹，不存在则返回默认值
func (m *FingerprintManager) GetFingerprint(tokenID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if fp, ok := m.fingerprints[tokenID]; ok {
		return fp
	}
	return KiroFingerprint // 返回默认指纹
}

// SetFingerprint 手动设置指纹（用于配置文件加载）
func (m *FingerprintManager) SetFingerprint(tokenID, fingerprint string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fingerprints[tokenID] = fingerprint
}

// GenerateFingerprintFromSeed 从种子生成确定性指纹
// 相同的种子总是生成相同的指纹
func GenerateFingerprintFromSeed(seed string) string {
	hash := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(hash[:])
}

// GenerateRandomFingerprint 生成随机指纹
func GenerateRandomFingerprint() string {
	data := fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
