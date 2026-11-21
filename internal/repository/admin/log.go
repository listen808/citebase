package admin

import (
	"database/sql"
	"errors"
	"time"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrLogNotFound = errors.New("log not found")
)

// LogRepository 日志数据访问层
type LogRepository struct {
	db *gorm.DB
}

func NewLogRepository() *LogRepository {
	return &LogRepository{db: repos.DB}
}

// CreateDocumentLog 创建文档操作日志
func (r *LogRepository) CreateDocumentLog(log *adminmodel.DocumentLog) error {
	log.CreateTime = int(time.Now().Unix())
	// Omit content 字段：该字段是 V2 新增列，旧版数据库可能没有此列
	// 且 LogDocumentAction 从未设置 Content，排除后兼容新旧数据库
	return r.db.Omit("content").Create(log).Error
}

// GetDocumentLogs 获取文档的操作历史
func (r *LogRepository) GetDocumentLogs(documentID int, limit int) ([]*adminmodel.DocumentLog, error) {
	var logs []*adminmodel.DocumentLog
	query := r.db.Where("document_id = ?", documentID).Order("create_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&logs).Error
	return logs, err
}

// GetUserDocumentLogs 获取用户在某文档的操作历史
// userID 为 0 时返回所有用户的活动记录
func (r *LogRepository) GetUserDocumentLogs(userID int, limit, offset int) ([]*adminmodel.DocumentLog, int64, error) {
	var logs []*adminmodel.DocumentLog
	var total int64

	query := r.db.Model(&adminmodel.DocumentLog{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	findQuery := r.db.Order("create_time DESC").Offset(offset).Limit(limit)
	if userID > 0 {
		findQuery = findQuery.Where("user_id = ?", userID)
	}
	err := findQuery.Find(&logs).Error
	return logs, total, err
}

// GetUserDocumentLogsFiltered 获取用户按操作类型筛选的活动记录
// userID 为 0 时返回所有用户的活动记录
func (r *LogRepository) GetUserDocumentLogsFiltered(userID int, actionFilter string, limit, offset int) ([]*adminmodel.DocumentLog, int64, error) {
	var logs []*adminmodel.DocumentLog
	var total int64

	var actionNum int
	switch actionFilter {
	case "create":
		actionNum = 1
	case "edit":
		actionNum = 2
	case "delete":
		actionNum = 3
	case "restore":
		actionNum = 4
	default:
		actionNum = 0
	}

	if actionNum == 0 {
		return r.GetUserDocumentLogs(userID, limit, offset)
	}

	query := r.db.Model(&adminmodel.DocumentLog{}).Where("action = ?", actionNum)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	findQuery := r.db.Where("action = ?", actionNum).Order("create_time DESC").Offset(offset).Limit(limit)
	if userID > 0 {
		findQuery = findQuery.Where("user_id = ?", userID)
	}
	err := findQuery.Find(&logs).Error
	return logs, total, err
}

// CreateSystemLog 创建系统操作日志
func (r *LogRepository) CreateSystemLog(log *adminmodel.SystemLog) error {
	log.CreateTime = int(time.Now().Unix())
	return r.db.Create(log).Error
}

// GetSystemLogs 获取系统日志（分页）
func (r *LogRepository) GetSystemLogs(level int, limit, offset int) ([]*adminmodel.SystemLog, int64, error) {
	var logs []*adminmodel.SystemLog
	var total int64

	query := r.db.Model(&adminmodel.SystemLog{})
	if level > 0 {
		query = query.Where("level = ?", level)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("create_time DESC").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

// CountByLevel 按日志级别统计数量（level 为 int：3=ERROR 4=WARN 6=INFO）
func (r *LogRepository) CountByLevel(level int) (int64, error) {
	var count int64
	query := r.db.Model(&adminmodel.SystemLog{})
	if level != 0 {
		query = query.Where("level = ?", level)
	}
	err := query.Count(&count).Error
	return count, err
}

// GetLogDetail 获取单条日志详情
func (r *LogRepository) GetLogDetail(logID int) (*adminmodel.SystemLog, error) {
	var log adminmodel.SystemLog
	err := r.db.First(&log, logID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrLogNotFound
		}
		return nil, err
	}
	return &log, nil
}

// GetRecentErrorLogs 获取最近错误日志（level=3）
func (r *LogRepository) GetRecentErrorLogs(limit int) ([]*adminmodel.SystemLog, error) {
	var logs []*adminmodel.SystemLog
	err := r.db.Where("level = ?", 3).
		Order("create_time DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
