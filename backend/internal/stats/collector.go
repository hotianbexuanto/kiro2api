package stats

import (
	"database/sql"
	"sync"
	"time"
)

const (
	workerCount = 3           // 写入 worker 数量
	queueSize   = 1000        // 队列大小
)

// ========== 新版：依赖注入 ==========

// Collector 统计收集器（依赖注入版本）
type Collector struct {
	db    *sql.DB
	queue chan RequestRecord
	once  sync.Once
}

// NewCollector 创建统计收集器
func NewCollector(db *sql.DB) *Collector {
	c := &Collector{
		db:    db,
		queue: make(chan RequestRecord, queueSize),
	}
	// 启动 worker
	for i := 0; i < workerCount; i++ {
		go c.worker()
	}
	return c
}

// Record 记录请求
func (c *Collector) Record(r RequestRecord) {
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now()
	}
	select {
	case c.queue <- r:
	default:
		// 队列满，丢弃
	}
}

// worker 处理写入
func (c *Collector) worker() {
	for r := range c.queue {
		c.persistRecord(r)
	}
}

// persistRecord 持久化记录
func (c *Collector) persistRecord(r RequestRecord) {
	if c.db == nil {
		return
	}
	// 复用现有的 persistRecord 逻辑
	persistRecordToDB(c.db, r)
}
