package ai

import (
	"errors"
	"fmt"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	repos "github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/text2sql"
	"github.com/listen808/citebase/internal/utils"
)

// 见 sql_trust_service.go 顶部同一条理由：句子进表，原因进日志。
var (
	ErrDataSourceNotFound      = errors.New("data source not found")
	ErrDataSourceConnectFailed = errors.New("failed to connect to the data source")
)

// DataSourceService 数据源管理业务逻辑层
type DataSourceService struct {
	repo *airepo.DataSourceRepo
}

// NewDataSourceService 创建 DataSourceService 实例
func NewDataSourceService(repo *airepo.DataSourceRepo) *DataSourceService {
	return &DataSourceService{repo: repo}
}

// CreateDataSourceRequest 创建数据源请求
type CreateDataSourceRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=200"`
	SpaceID  uint64 `json:"space_id" binding:"required"`
	DBType   string `json:"db_type" binding:"required,oneof=mysql postgres oracle"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required"`
	DBName   string `json:"db_name" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	SyncCron string `json:"sync_cron"`
	IsActive bool   `json:"is_active"`
}

// Create 创建数据源
func (s *DataSourceService) Create(req *CreateDataSourceRequest) (*aimodel.DataSource, error) {
	// 口令入库前加密：这条 DSN 里的凭据此前是明文写进 aw_data_source 的，
	// 而 make db-dump 与「系统管理 → 备份」都会把整库导出成 SQL 文件 ——
	// 于是一份备份就带着所有已配置数据库的口令，备份文件的存放级别成了唯一的防线。
	// 读取侧用 DecryptSecretOrPlain，历史明文行仍能直接用（见 utils/crypto.go）。
	sealed, err := utils.EncryptSecret(req.Password)
	if err != nil {
		return nil, fmt.Errorf("加密数据源口令失败: %w", err)
	}
	ds := &aimodel.DataSource{
		Name:     req.Name,
		SpaceID:  req.SpaceID,
		DBType:   req.DBType,
		Host:     req.Host,
		Port:     req.Port,
		DBName:   req.DBName,
		Username: req.Username,
		Password: sealed,
		SyncCron: req.SyncCron,
		IsActive: req.IsActive,
	}
	if err := s.repo.Create(ds); err != nil {
		return nil, err
	}
	return ds, nil
}

// Update 更新数据源
func (s *DataSourceService) Update(id uint64, req *CreateDataSourceRequest) (*aimodel.DataSource, error) {
	ds, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDataSourceNotFound, err)
	}
	ds.Name = req.Name
	ds.DBType = req.DBType
	ds.Host = req.Host
	ds.Port = req.Port
	ds.DBName = req.DBName
	ds.Username = req.Username
	ds.SyncCron = req.SyncCron
	ds.IsActive = req.IsActive

	// 口令这一栏有两种"没填"的含义，必须分开：
	//   - 空串：管理页只是没改口令（表单回填时口令栏被清掉了），保留库里已有的值。
	//     原先无条件 ds.Password = req.Password，等于"只改端口就把口令抹掉"，
	//     下一次同步报连接失败时用户完全看不出是自己刚保存造成的。
	//   - 看着像密文：列表接口虽然把 Password 清成 ""，但任何把整条记录原样回传的实现
	//     都可能带回来，再加密一次就成了"密文的密文"，永远解不回明文。
	switch {
	case req.Password == "":
		// 保留旧值
	case utils.LooksEncrypted(req.Password):
		logx.Warnf("[DataSource] 更新数据源 %d 时提交的口令已是密文形态，按未修改处理", id)
	default:
		sealed, err := utils.EncryptSecret(req.Password)
		if err != nil {
			return nil, fmt.Errorf("加密数据源口令失败: %w", err)
		}
		ds.Password = sealed
	}

	if err := s.repo.Update(ds); err != nil {
		return nil, err
	}
	return ds, nil
}

// Delete 删除数据源
func (s *DataSourceService) Delete(id uint64) error {
	return s.repo.Delete(id)
}

// List 列出数据源（隐藏密码）
func (s *DataSourceService) List(spaceID uint64) ([]aimodel.DataSource, error) {
	list, err := s.repo.FindBySpace(spaceID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Password = ""
	}
	return list, nil
}

// Get 获取数据源详情（隐藏密码）
func (s *DataSourceService) Get(id uint64) (*aimodel.DataSource, error) {
	ds, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDataSourceNotFound, err)
	}
	ds.Password = ""
	return ds, nil
}

// TestConnectionRequest 测试连接请求
type TestConnectionRequest struct {
	DBType   string `json:"db_type" binding:"required"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required"`
	DBName   string `json:"db_name" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// TestConnection 测试数据库连接
func (s *DataSourceService) TestConnection(req *TestConnectionRequest) error {
	cfg := text2sql.DBConfig{
		DBType:   req.DBType,
		Host:     req.Host,
		Port:     req.Port,
		DBName:   req.DBName,
		Username: req.Username,
		Password: req.Password,
	}
	conn, err := text2sql.NewDBConnector(cfg)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDataSourceConnectFailed, err)
	}
	if err := conn.Connect(); err != nil {
		return fmt.Errorf("%w: %w", ErrDataSourceConnectFailed, err)
	}
	defer conn.Close()
	if err := conn.Ping(); err != nil {
		return fmt.Errorf("%w: %w", ErrDataSourceConnectFailed, err)
	}
	return nil
}

// SyncResult 同步结果
type SyncResult struct {
	Message    string `json:"message"`
	TableCount int    `json:"table_count"`
}

// Sync 同步数据源表结构
func (s *DataSourceService) Sync(id uint64) (*SyncResult, error) {
	ds, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDataSourceNotFound, err)
	}
	cfg := text2sql.DBConfig{
		DBType:   ds.DBType,
		Host:     ds.Host,
		Port:     ds.Port,
		DBName:   ds.DBName,
		Username: ds.Username,
		Password: utils.DecryptSecretOrPlain(ds.Password),
		// Timeout 是 time.Duration（纳秒为单位），旧写法 30 会被当成 30ns，
		// DSN 里生成 timeout=30ns，每次建连必然超时失败。
		Timeout: 30 * time.Second,
	}
	conn, err := text2sql.NewDBConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("同步失败: %w", err)
	}
	if err := conn.Connect(); err != nil {
		return nil, fmt.Errorf("同步失败: %w", err)
	}
	defer conn.Close()

	tables, err := conn.FetchAllDDL()
	if err != nil {
		return nil, fmt.Errorf("同步失败: %w", err)
	}
	// 旧实现只统计 len(tables) 就宣称同步成功，DDL 从未落库，
	// Text2SQL 依赖的表缓存永远不会随数据源更新。这里把结果持久化到 TableCache。
	tableCacheRepo := airepo.NewTextToSQLRepo(repos.DB)
	spaceID := ds.SpaceID
	for _, t := range tables {
		if t == nil {
			continue
		}
		t.SpaceID = &spaceID
		if t.DBType == "" {
			t.DBType = ds.DBType
		}
		if err := tableCacheRepo.UpsertTableCache(t); err != nil {
			return nil, fmt.Errorf("同步失败: 写入表缓存 %s 出错: %w", t.TableNam, err)
		}
	}
	// 同步时间戳写入失败也要上抛，否则用户看到成功回执但 last_sync_at 永远不更新
	if err := s.repo.UpdateLastSync(id); err != nil {
		return nil, fmt.Errorf("同步失败: 更新同步时间出错: %w", err)
	}
	return &SyncResult{Message: "同步成功", TableCount: len(tables)}, nil
}
