package xfs

import (
	"fmt"
	"sync"
)

// ProjectIDPool 管理项目 ID 的分配
type ProjectIDPool struct {
	used  map[uint32]bool
	mutex sync.Mutex
	minID uint32
	maxID uint32
}

// NewProjectIDPool 创建项目 ID 池
func NewProjectIDPool(minID, maxID uint32) *ProjectIDPool {
	return &ProjectIDPool{
		used:  make(map[uint32]bool),
		minID: minID,
		maxID: maxID,
	}
}

// Allocate 分配一个未使用的项目 ID
func (p *ProjectIDPool) Allocate() (uint32, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for id := p.minID; id <= p.maxID; id++ {
		if !p.used[id] {
			p.used[id] = true
			return id, nil
		}
	}
	return 0, fmt.Errorf("no available project ID")
}

// Release 释放项目 ID
func (p *ProjectIDPool) Release(id uint32) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	delete(p.used, id)
}

// MarkUsed 标记项目 ID 为已使用
func (p *ProjectIDPool) MarkUsed(id uint32) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.used[id] = true
}
