package xfs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// State 存储容器 ID 与项目 ID 和 upperdir 的映射
type State struct {
	Entries map[string]Entry `json:"entries"`
}

// Entry 表示单条映射
type Entry struct {
	ContainerID string `json:"container_id"`
	ProjectID   uint32 `json:"project_id"`
	Upperdir    string `json:"upperdir"`
}

// StateManager 管理状态的并发安全结构
type StateManager struct {
	filePath string
	state    State
	mutex    sync.RWMutex
}

// NewStateManager 创建状态管理器
func NewStateManager(filePath string) (*StateManager, error) {
	m := &StateManager{
		filePath: filePath,
		state:    State{Entries: make(map[string]Entry)},
	}

	if err := m.load(); err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，创建父目录和文件
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return nil, err
			}
			file, err := os.Create(filePath)
			if err != nil {
				return nil, err
			}
			file.Close()
		} else {
			// 其他加载错误
			return nil, err
		}
	}
	return m, nil
}

// load 从文件加载状态
func (m *StateManager) load() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}
	if len(data) == 0 || len(m.state.Entries) != 0 {
		return nil
	}
	return json.Unmarshal(data, &m.state)
}

// save 保存状态到文件
func (m *StateManager) save() error {

	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0644)
}

// AddEntry 添加或更新映射
func (m *StateManager) AddEntry(containerID string, projectID uint32, upperdir string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.state.Entries[containerID] = Entry{
		ContainerID: containerID,
		ProjectID:   projectID,
		Upperdir:    upperdir,
	}
	return m.save()
}

// RemoveEntry 删除映射
func (m *StateManager) RemoveEntry(containerID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.state.Entries, containerID)
	return m.save()
}

// GetEntry 获取映射
func (m *StateManager) GetEntry(containerID string) (Entry, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	entry, exists := m.state.Entries[containerID]
	return entry, exists
}
