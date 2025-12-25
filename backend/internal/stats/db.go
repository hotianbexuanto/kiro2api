package stats

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"kiro2api/internal/logger"

	_ "modernc.org/sqlite"
)

var (
	logDB     *sql.DB
	logDBOnce sync.Once
	logDBErr  error
)

const logSchema = `
CREATE TABLE IF NOT EXISTS request_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT,
    timestamp DATETIME,
    method TEXT,
    path TEXT,
    request_type TEXT,
    model TEXT,
    stream INTEGER,
    status_code INTEGER,
    latency_ms INTEGER,
    ttfb_ms INTEGER,
    credit_usage REAL,
    context_usage_percent REAL,
    actual_input_tokens INTEGER,
    calculated_output_tokens INTEGER,
    cache_hit INTEGER,
    token_index INTEGER,
    conversation_id TEXT,
    group_name TEXT,
    error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON request_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_logs_model ON request_logs(model);
CREATE INDEX IF NOT EXISTS idx_logs_group ON request_logs(group_name);
CREATE INDEX IF NOT EXISTS idx_logs_request_id ON request_logs(request_id);
`

// InitLogDB 初始化请求日志数据库
func InitLogDB(dbPath string) error {
	logDBOnce.Do(func() {
		// 确保目录存在
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			logDBErr = err
			return
		}

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			logDBErr = err
			return
		}

		// 设置连接池（WAL 模式支持 1写+多读并发）
		db.SetMaxOpenConns(5)            // 允许 5 个并发连接
		db.SetMaxIdleConns(2)            // 保持 2 个空闲连接
		db.SetConnMaxLifetime(time.Hour) // 连接最大生命周期

		// 创建表
		if _, err := db.Exec(logSchema); err != nil {
			logDBErr = err
			db.Close()
			return
		}

		logDB = db
		logger.Info("请求日志数据库初始化完成", logger.String("path", dbPath))
	})
	return logDBErr
}

// GetLogDB 获取日志数据库连接
func GetLogDB() *sql.DB {
	return logDB
}

// CloseLogDB 关闭日志数据库
func CloseLogDB() {
	if logDB != nil {
		logDB.Close()
	}
}

// persistRecordToDB 持久化记录到指定数据库（依赖注入版本）
func persistRecordToDB(db *sql.DB, r RequestRecord) {
	// 计算 actual_input_tokens 和 calculated_output_tokens
	actualInputTokens := int(r.ContextUsagePercent / 100 * 200000)
	var calculatedOutputTokens int
	var cacheHit int

	if r.CreditUsage > 0 && actualInputTokens > 0 {
		calculatedOutputTokens = int((r.CreditUsage*1000000 - float64(actualInputTokens)*3) / 15)
		if calculatedOutputTokens < 0 {
			calculatedOutputTokens = 0
		}
		// 检测缓存
		expectedCredit := float64(actualInputTokens)*3/1000000 + float64(r.OutputTokens)*15/1000000
		if r.CreditUsage < expectedCredit*0.5 {
			cacheHit = 1
		}
	}

	const insertSQL = `
		INSERT INTO request_logs (
			request_id, timestamp, method, path, request_type, model, stream,
			status_code, latency_ms, ttfb_ms, credit_usage, context_usage_percent,
			actual_input_tokens, calculated_output_tokens, cache_hit,
			token_index, conversation_id, group_name, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// 转换 timestamp 为 SQLite 可识别的格式 (RFC3339)
	timestampStr := r.Timestamp.Format(time.RFC3339)

	_, err := db.Exec(insertSQL,
		r.ID, timestampStr, r.Method, r.Path, r.RequestType, r.Model, r.Stream,
		r.StatusCode, r.Latency, r.TTFB, r.CreditUsage, r.ContextUsagePercent,
		actualInputTokens, calculatedOutputTokens, cacheHit,
		r.TokenIndex, r.ConversationId, r.Group, r.Error,
	)
	if err != nil {
		logger.Error("写入请求日志失败", logger.Err(err))
	}
}

// === 查询函数 ===

type queryLogsParams struct {
	Page     int
	PageSize int
	Model    string
	Group    string
	DateFrom string
	DateTo   string
}

type queryLogsResult struct {
	Total   int64       `json:"total"`
	Page    int         `json:"page"`
	Pages   int         `json:"pages"`
	Records []logRecord `json:"records"`
}

type logRecord struct {
	ID                     int64   `json:"id"`
	RequestID              string  `json:"request_id"`
	Timestamp              string  `json:"timestamp"`
	Model                  string  `json:"model"`
	Stream                 bool    `json:"stream"`
	StatusCode             int     `json:"status_code"`
	LatencyMs              int64   `json:"latency_ms"`
	ActualInputTokens      int     `json:"actual_input_tokens"`
	CalculatedOutputTokens int     `json:"calculated_output_tokens"`
	CreditUsage            float64 `json:"credit_usage"`
	ContextUsagePercent    float64 `json:"context_usage_percent"`
	CacheHit               bool    `json:"cache_hit"`
	TokenIndex             int     `json:"token_index"`
	ConversationId         string  `json:"conversation_id"`
	Group                  string  `json:"group"`
}

func queryLogs(params queryLogsParams) (*queryLogsResult, error) {
	db := GetLogDB()
	if db == nil {
		return nil, fmt.Errorf("日志数据库未初始化")
	}

	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 100
	}

	where := "1=1"
	args := []any{}

	if params.Model != "" {
		where += " AND model = ?"
		args = append(args, params.Model)
	}
	if params.Group != "" {
		where += " AND group_name = ?"
		args = append(args, params.Group)
	}
	if params.DateFrom != "" {
		where += " AND timestamp >= ?"
		args = append(args, params.DateFrom+" 00:00:00")
	}
	if params.DateTo != "" {
		where += " AND timestamp <= ?"
		args = append(args, params.DateTo+" 23:59:59")
	}

	var total int64
	db.QueryRow("SELECT COUNT(*) FROM request_logs WHERE "+where, args...).Scan(&total)

	offset := (params.Page - 1) * params.PageSize
	pages := int((total + int64(params.PageSize) - 1) / int64(params.PageSize))

	querySQL := `SELECT id, request_id, timestamp, model, stream, status_code, latency_ms,
		actual_input_tokens, calculated_output_tokens,
		credit_usage, context_usage_percent, cache_hit, token_index, conversation_id, group_name
		FROM request_logs WHERE ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, params.PageSize, offset)

	rows, err := db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := []logRecord{}
	for rows.Next() {
		var r logRecord
		var cacheHit, stream int
		rows.Scan(&r.ID, &r.RequestID, &r.Timestamp, &r.Model, &stream, &r.StatusCode, &r.LatencyMs,
			&r.ActualInputTokens, &r.CalculatedOutputTokens,
			&r.CreditUsage, &r.ContextUsagePercent, &cacheHit, &r.TokenIndex, &r.ConversationId, &r.Group)
		r.CacheHit = cacheHit == 1
		r.Stream = stream == 1
		records = append(records, r)
	}

	return &queryLogsResult{Total: total, Page: params.Page, Pages: pages, Records: records}, nil
}

func clearLogs(daysToKeep int) (int64, error) {
	db := GetLogDB()
	if db == nil {
		return 0, fmt.Errorf("日志数据库未初始化")
	}
	cutoff := time.Now().AddDate(0, 0, -daysToKeep)
	result, err := db.Exec("DELETE FROM request_logs WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func clearAllLogs() error {
	db := GetLogDB()
	if db == nil {
		return fmt.Errorf("日志数据库未初始化")
	}
	_, err := db.Exec("DELETE FROM request_logs")
	if err != nil {
		return err
	}
	_, err = db.Exec("VACUUM")
	return err
}

func getLogStats() (map[string]any, error) {
	db := GetLogDB()
	if db == nil {
		return nil, fmt.Errorf("日志数据库未初始化")
	}

	stats := map[string]any{}

	// 单个查询获取所有统计
	var total int64
	var oldest, newest sql.NullString
	var totalCredit float64
	err := db.QueryRow(`
		SELECT
			COUNT(*),
			MIN(timestamp),
			MAX(timestamp),
			COALESCE(SUM(credit_usage), 0)
		FROM request_logs
	`).Scan(&total, &oldest, &newest, &totalCredit)
	if err != nil {
		return nil, fmt.Errorf("查询日志统计失败: %w", err)
	}

	stats["total_records"] = total
	stats["total_credit_usage"] = totalCredit
	if oldest.Valid {
		stats["oldest_record"] = oldest.String
	}
	if newest.Valid {
		stats["newest_record"] = newest.String
	}

	// 数据库文件大小
	logDBPath := os.Getenv("KIRO_LOG_DB_PATH")
	if logDBPath == "" {
		logDBPath = "./data/request_logs.db"
	}
	if fi, err := os.Stat(logDBPath); err == nil {
		stats["db_size_mb"] = float64(fi.Size()) / 1024 / 1024
	}

	return stats, nil
}

// TokenStats 单个 token 的聚合统计
type TokenStats struct {
	TokenIndex   int
	RequestCount int64
	SuccessCount int64
	FailureCount int64
	TotalLatency int64 // 毫秒
}

// GetTokenStats 从数据库查询每个 token 的聚合统计
func GetTokenStats() (map[int]*TokenStats, error) {
	db := GetLogDB()
	if db == nil {
		return nil, fmt.Errorf("日志数据库未初始化")
	}

	result := make(map[int]*TokenStats)

	rows, err := db.Query(`
		SELECT token_index,
			COUNT(*) as request_count,
			SUM(CASE WHEN status_code < 400 THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as failure_count,
			COALESCE(SUM(latency_ms), 0) as total_latency
		FROM request_logs
		GROUP BY token_index
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var s TokenStats
		rows.Scan(&s.TokenIndex, &s.RequestCount, &s.SuccessCount, &s.FailureCount, &s.TotalLatency)
		result[s.TokenIndex] = &s
	}

	return result, nil
}

// getRecentRecords 从数据库查询最近的记录用于恢复
func getRecentRecords(limit int) ([]RequestRecord, error) {
	db := GetLogDB()
	if db == nil {
		return nil, fmt.Errorf("日志数据库未初始化")
	}

	rows, err := db.Query(`
		SELECT request_id, timestamp, method, path, request_type, model, stream,
			status_code, latency_ms, ttfb_ms, credit_usage, context_usage_percent,
			token_index, conversation_id, group_name, error
		FROM request_logs
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []RequestRecord
	for rows.Next() {
		var r RequestRecord
		var ts string
		var stream int
		var ttfb, convId, errStr sql.NullString
		var reqType sql.NullString

		rows.Scan(&r.ID, &ts, &r.Method, &r.Path, &reqType, &r.Model, &stream,
			&r.StatusCode, &r.Latency, &ttfb, &r.CreditUsage, &r.ContextUsagePercent,
			&r.TokenIndex, &convId, &r.Group, &errStr)

		// 尝试RFC3339格式(新格式)，失败则尝试Go String格式(旧格式)
		r.Timestamp, _ = time.Parse(time.RFC3339, ts)
		if r.Timestamp.IsZero() {
			r.Timestamp, _ = time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", ts)
		}
		r.Stream = stream == 1
		if reqType.Valid {
			r.RequestType = reqType.String
		}
		if convId.Valid {
			r.ConversationId = convId.String
		}
		if errStr.Valid {
			r.Error = errStr.String
		}

		records = append(records, r)
	}

	return records, nil
}

// GetStatsFromDB 从数据库查询完整统计（唯一数据源）
func GetStatsFromDB() (*Stats, error) {
	db := GetLogDB()
	if db == nil {
		return nil, fmt.Errorf("日志数据库未初始化")
	}

	stats := &Stats{
		ModelUsage:    make(map[string]int64),
		PathUsage:     make(map[string]int64),
		HourlyByModel: make(map[string][24]int64),
	}

	// 单个查询获取所有基础统计
	err := db.QueryRow(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status_code < 400 THEN 1 ELSE 0 END) as success,
			COALESCE(SUM(latency_ms), 0) as total_latency
		FROM request_logs
	`).Scan(&stats.TotalRequests, &stats.SuccessRequests, &stats.AvgLatency)
	if err != nil {
		return nil, fmt.Errorf("查询基础统计失败: %w", err)
	}

	stats.FailedRequests = stats.TotalRequests - stats.SuccessRequests
	if stats.TotalRequests > 0 {
		stats.AvgLatency = stats.AvgLatency / float64(stats.TotalRequests)
	}

	// 模型使用统计
	rows, err := db.Query("SELECT model, COUNT(*) FROM request_logs GROUP BY model")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var model string
			var count int64
			if err := rows.Scan(&model, &count); err == nil {
				stats.ModelUsage[model] = count
			}
		}
	}

	// 按小时请求统计 (WHERE datetime(timestamp) IS NOT NULL 排除无效时间戳)
	rows, err = db.Query(`
		SELECT strftime('%H', datetime(timestamp)) as hour, COUNT(*) as count
		FROM request_logs
		WHERE datetime(timestamp) IS NOT NULL
		GROUP BY hour
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var hourStr string
			var count int64
			if err := rows.Scan(&hourStr, &count); err == nil {
				hour, _ := strconv.Atoi(hourStr)
				if hour >= 0 && hour < 24 {
					stats.HourlyRequests[hour] = count
				}
			}
		}
	}

	// 按小时credit统计
	rows, err = db.Query(`
		SELECT strftime('%H', datetime(timestamp)) as hour, SUM(credit_usage) as total_credit
		FROM request_logs
		WHERE datetime(timestamp) IS NOT NULL
		GROUP BY hour
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var hourStr string
			var credit float64
			if err := rows.Scan(&hourStr, &credit); err == nil {
				hour, _ := strconv.Atoi(hourStr)
				if hour >= 0 && hour < 24 {
					stats.HourlyCredit[hour] = credit
				}
			}
		}
	}

	// 按小时按模型统计
	rows, err = db.Query(`
		SELECT model, strftime('%H', datetime(timestamp)) as hour, COUNT(*) as count
		FROM request_logs
		WHERE datetime(timestamp) IS NOT NULL
		GROUP BY model, hour
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var model, hourStr string
			var count int64
			if err := rows.Scan(&model, &hourStr, &count); err == nil {
				hour, _ := strconv.Atoi(hourStr)
				if hour >= 0 && hour < 24 {
					if _, exists := stats.HourlyByModel[model]; !exists {
						stats.HourlyByModel[model] = [24]int64{}
					}
					hourlyData := stats.HourlyByModel[model]
					hourlyData[hour] = count
					stats.HourlyByModel[model] = hourlyData
				}
			}
		}
	}

	// 最近记录
	records, _ := getRecentRecords(1000)
	stats.RecentRecords = records

	return stats, nil
}
