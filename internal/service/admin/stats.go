package admin

import (
	"fmt"
	"runtime"

	logx "github.com/listen808/citebase/internal/log"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
	socialrepo "github.com/listen808/citebase/internal/repository/social"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/version"

	"time"

	searchsvc "github.com/listen808/citebase/internal/service/search"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type StatsService struct {
	userRepo   *userrepo.UserRepository
	spaceRepo  *workspacerepo.SpaceRepository
	docRepo    *workspacerepo.DocumentRepository
	followRepo *socialrepo.FollowRepository
	logRepo    *adminrepo.LogRepository
	colRepo    *socialrepo.CollectionRepository
}

func NewStatsService() *StatsService {
	return &StatsService{
		userRepo:   userrepo.NewUserRepository(),
		spaceRepo:  workspacerepo.NewSpaceRepository(),
		docRepo:    workspacerepo.NewDocumentRepository(),
		followRepo: socialrepo.NewFollowRepository(),
		logRepo:    adminrepo.NewLogRepository(),
		colRepo:    socialrepo.NewCollectionRepository(),
	}
}

func (s *StatsService) GetDashboardStats(days int) (*adminmodel.DashboardStats, error) {
	stats := &adminmodel.DashboardStats{}

	normalCount, _ := s.userRepo.CountNormalUsers()
	stats.TotalUsers = normalCount

	forbiddenCount, _ := s.userRepo.CountForbiddenUsers()
	stats.ForbiddenUsers = forbiddenCount

	onlineUsers, _ := s.userRepo.CountOnlineUsers()
	stats.ActiveUsers = onlineUsers

	spaceCount, _ := s.spaceRepo.CountAll()
	stats.TotalSpaces = spaceCount

	docCount, _ := s.docRepo.CountAll()
	stats.TotalDocuments = docCount

	todayDocs, _ := s.docRepo.CountTodayDocuments()
	stats.TodayDocuments = todayDocs

	indexStats, _ := searchsvc.GetIndexStats()
	if indexedCount, ok := indexStats["doc_count"].(int64); ok {
		stats.IndexedDocuments = indexedCount
		if indexedCount > 0 && indexedCount == stats.TotalDocuments {
			stats.IndexStatus = "正常"
		} else if indexedCount > 0 {
			stats.IndexStatus = "部分索引"
		} else {
			stats.IndexStatus = "待索引"
		}
	} else {
		stats.IndexedDocuments = 0
		stats.IndexStatus = "待索引"
	}

	stats.TopCreator = s.userRepo.GetTopCreator()
	stats.TopEditor = s.userRepo.GetTopEditor()
	stats.TopCollector = s.spaceRepo.GetTopCollector()
	stats.TopFollower = s.followRepo.GetTopFollower()

	if contributors, err := s.userRepo.GetActiveContributors(10); err == nil {
		stats.ActiveContributors = contributors
	}

	stats.SpaceRank = s.spaceRepo.GetSpaceRank(15)

	if trend, err := s.docRepo.GetDocCountByTime(days); err == nil {
		stats.DocumentTrend = trend
	}

	if hotDocs, err := s.docRepo.GetHotDocuments(10); err == nil {
		stats.HotDocuments = hotDocs
	}

	return stats, nil
}

func (s *StatsService) GetServerStatus() (*adminmodel.ServerStatus, error) {
	status := &adminmodel.ServerStatus{
		LocalIP: "localhost",
	}

	if err := s.loadSystemStatus(status); err != nil {
		status.CPUPercent = 0
		status.MemoryPercent = 0
		status.MemoryUsed = 0
		status.MemoryTotal = 0
		status.DiskPercent = 0
		status.DiskUsed = 0
		status.DiskTotal = 0
	}

	errorCount, _ := s.logRepo.CountByLevel(3)
	status.ErrorLogCount = errorCount

	build := version.BuildInfo()
	status.Version = build.Version
	status.Commit = build.Commit
	status.GoVersion = build.GoVersion

	return status, nil
}

func (s *StatsService) GetServerTime() *adminmodel.ServerTime {
	return &adminmodel.ServerTime{
		ServerTime: time.Now().Unix(),
		RunTime:    int64(time.Since(startTime).Seconds()),
	}
}

func (s *StatsService) loadSystemStatus(status *adminmodel.ServerStatus) error {
	vm, err := mem.VirtualMemory()
	if err == nil {
		status.MemoryPercent = vm.UsedPercent
		status.MemoryUsed = vm.Used
		status.MemoryTotal = vm.Total
	}

	cpuPercent, err := cpu.Percent(200*time.Millisecond, false)
	if err == nil && len(cpuPercent) > 0 {
		status.CPUPercent = cpuPercent[0]
	}

	// 挂载点按实际部署 OS 决定：Linux/容器用 "/"，Windows 用系统盘 "C:"。
	// 旧实现以 LocalIP=="localhost"（此处恒为该值）判断，Linux 上 disk.Usage("C:") 必然失败，
	// 而错误又被吞掉，磁盘使用率永远显示 0、容量告警失效。
	mountPoint := "/"
	if runtime.GOOS == "windows" {
		mountPoint = "C:"
	}
	d, err := disk.Usage(mountPoint)
	if err != nil {
		// 携带采集失败标记，让运维页能区分「真的是 0」与「采集失败」
		status.SystemStatusError = fmt.Sprintf("disk usage collect failed (%s): %v", mountPoint, err)
		logx.Errorf("[Stats] %s", status.SystemStatusError)
	} else {
		status.DiskPercent = d.UsedPercent
		status.DiskUsed = d.Used
		status.DiskTotal = d.Total
	}

	hostInfo, err := host.Info()
	if err == nil {
		status.Hostname = hostInfo.Hostname
		status.OS = hostInfo.OS
		status.Platform = hostInfo.Platform
		status.PlatformFamily = hostInfo.PlatformFamily
		status.Uptime = int64(hostInfo.Uptime)
	}

	status.NetworkPercent = 0

	return nil
}

var startTime = time.Now()

func (s *StatsService) GetCollectDocRank(limit int) ([]socialrepo.DocCollectItem, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.colRepo.GetCollectDocRank(limit)
}
