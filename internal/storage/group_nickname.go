package storage

import (
	"QQBot/internal/common"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// GroupNicknameMap 群昵称映射（持久化）
type GroupNicknameMap struct {
	GroupID   int64            `json:"group_id"`
	Nicknames map[int64]string `json:"nicknames"` // QQ号 -> 昵称
	mu        sync.RWMutex
}

var (
	// 昵称映射（按群分开，持久化）
	groupNicknameMap sync.Map // map[int64]*GroupNicknameMap
)

// UpdateNicknameMap 更新昵称映射（按群分开，自动保存）
func UpdateNicknameMap(groupID int64, userID int64, nickname string) {
	if nickname == "" || groupID == 0 {
		return
	}

	nm := getOrCreateGroupNicknameMap(groupID)
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// 检查是否有变化
	if oldNickname, exists := nm.Nicknames[userID]; exists && oldNickname == nickname {
		return // 没有变化，不需要保存
	}

	nm.Nicknames[userID] = nickname

	// 异步保存到文件
	go nm.saveToFile()
}

// GetNickname 获取用户昵称，如果不存在则返回稳定标识符
// groupID=0 表示私聊，直接返回稳定标识符（私聊不需要昵称）
// 注意：不再在昵称后追加身份标识，身份由 GetRoleTag() 单独提供
func GetNickname(groupID int64, userID int64) string {
	if userID == common.BotQQNumber || userID == 0 {
		return "小牛"
	}
	if groupID == 0 {
		// 私聊直接返回稳定标识符
		return getUserStableID(userID)
	}

	// 获取该群的昵称映射
	nm := getOrCreateGroupNicknameMap(groupID)
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	if n, ok := nm.Nicknames[userID]; ok && n != "" {
		return n
	}
	return getUserStableID(userID)
}

// getOrCreateGroupNicknameMap 获取或创建群的昵称映射（从文件加载）
func getOrCreateGroupNicknameMap(groupID int64) *GroupNicknameMap {
	// 先从内存中查找
	if mapInterface, ok := groupNicknameMap.Load(groupID); ok {
		return mapInterface.(*GroupNicknameMap)
	}

	// 尝试从文件加载
	nm := loadGroupNicknameMapFromFile(groupID)
	if nm == nil {
		// 创建新的昵称映射
		nm = &GroupNicknameMap{
			GroupID:   groupID,
			Nicknames: make(map[int64]string),
		}
	}

	// 存储到内存
	groupNicknameMap.Store(groupID, nm)
	return nm
}

// saveToFile 保存群昵称映射到文件
func (nm *GroupNicknameMap) saveToFile() {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// 确保目录存在
	if err := os.MkdirAll(HistoryDataDir, 0755); err != nil {
		log.Printf("[昵称映射] 创建目录失败: %v", err)
		return
	}

	filename := filepath.Join(HistoryDataDir, fmt.Sprintf("group_%d_nicknames.json", nm.GroupID))
	data, err := json.MarshalIndent(nm, "", "  ")
	if err != nil {
		log.Printf("[昵称映射] 序列化失败: %v", err)
		return
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Printf("[昵称映射] 保存文件失败: %v", err)
	}
}

// loadGroupNicknameMapFromFile 从文件加载群昵称映射
func loadGroupNicknameMapFromFile(groupID int64) *GroupNicknameMap {
	filename := filepath.Join(HistoryDataDir, fmt.Sprintf("group_%d_nicknames.json", groupID))

	data, err := os.ReadFile(filename)
	if err != nil {
		// 文件不存在是正常的
		return nil
	}

	var nm GroupNicknameMap
	if err := json.Unmarshal(data, &nm); err != nil {
		log.Printf("[昵称映射] 加载文件失败: %v", err)
		return nil
	}

	return &nm
}
