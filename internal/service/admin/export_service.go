package admin

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	userrepo "github.com/listen808/citebase/internal/repository/user"
	docrepo "github.com/listen808/citebase/internal/repository/workspace"
)

func formatUnix(ts int) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(int64(ts), 0).Format(time.RFC3339)
}

// ExportService 统一数据导出服务（G037）
type ExportService struct {
	docRepo   *docrepo.DocumentRepository
	spaceRepo *docrepo.SpaceRepository
	userRepo  *userrepo.UserRepository
}

// NewExportService 创建导出服务
func NewExportService() *ExportService {
	return &ExportService{
		docRepo:   docrepo.NewDocumentRepository(),
		spaceRepo: docrepo.NewSpaceRepository(),
		userRepo:  userrepo.NewUserRepository(),
	}
}

// SupportedModules 返回支持的导出模块
func (s *ExportService) SupportedModules() []string {
	return []string{"documents", "spaces", "users"}
}

// ExportResult 导出结果
type ExportResult struct {
	Module   string `json:"module"`
	Format   string `json:"format"`
	Filename string `json:"filename"`
}

// ExportCSV 导出指定模块为 CSV
func (s *ExportService) ExportCSV(module string) ([]byte, *ExportResult, error) {
	switch module {
	case "documents":
		return s.exportDocumentsCSV()
	case "spaces":
		return s.exportSpacesCSV()
	case "users":
		return s.exportUsersCSV()
	default:
		return nil, nil, fmt.Errorf("unsupported module: %s (available: %s)", module, strings.Join(s.SupportedModules(), ", "))
	}
}

// ExportJSON 导出指定模块为 JSON
func (s *ExportService) ExportJSON(module string) ([]byte, *ExportResult, error) {
	switch module {
	case "documents":
		return s.exportDocumentsJSON()
	case "spaces":
		return s.exportSpacesJSON()
	case "users":
		return s.exportUsersJSON()
	default:
		return nil, nil, fmt.Errorf("unsupported module: %s (available: %s)", module, strings.Join(s.SupportedModules(), ", "))
	}
}

// exportDocumentsCSV 导出文档列表为 CSV
func (s *ExportService) exportDocumentsCSV() ([]byte, *ExportResult, error) {
	docs, err := s.docRepo.FindAllDocuments()
	if err != nil {
		return nil, nil, err
	}
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"ID", "名称", "空间ID", "父级ID", "类型", "创建者ID", "状态", "创建时间", "更新时间"})
	for _, d := range docs {
		_ = w.Write([]string{
			fmt.Sprintf("%d", d.ID),
			d.Name,
			fmt.Sprintf("%d", d.SpaceID),
			fmt.Sprintf("%d", d.ParentID),
			fmt.Sprintf("%d", d.Type),
			fmt.Sprintf("%d", d.CreateUserID),
			d.Status,
			formatUnix(d.CreateTime),
			formatUnix(d.UpdateTime),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, nil, err
	}
	return []byte(buf.String()), &ExportResult{Module: "documents", Format: "csv", Filename: "documents.csv"}, nil
}

// exportDocumentsJSON 导出文档列表为 JSON
func (s *ExportService) exportDocumentsJSON() ([]byte, *ExportResult, error) {
	docs, err := s.docRepo.FindAllDocuments()
	if err != nil {
		return nil, nil, err
	}
	type docJSON struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		SpaceID    int    `json:"space_id"`
		ParentID   int    `json:"parent_id"`
		Type       int    `json:"type"`
		CreateUser int    `json:"create_user_id"`
		Status     string `json:"status"`
		CreateTime string `json:"create_time"`
		UpdateTime string `json:"update_time"`
	}
	out := make([]docJSON, len(docs))
	for i, d := range docs {
		out[i] = docJSON{
			ID: d.ID, Name: d.Name, SpaceID: d.SpaceID, ParentID: d.ParentID,
			Type: d.Type, CreateUser: d.CreateUserID, Status: d.Status,
			CreateTime: formatUnix(d.CreateTime), UpdateTime: formatUnix(d.UpdateTime),
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return b, &ExportResult{Module: "documents", Format: "json", Filename: "documents.json"}, nil
}

// exportSpacesCSV 导出空间列表为 CSV
func (s *ExportService) exportSpacesCSV() ([]byte, *ExportResult, error) {
	spaces, total, err := s.spaceRepo.FindAll(0, 10000, "", "", "")
	if err != nil {
		return nil, nil, err
	}
	// 导出是合规/迁移依据：旧实现静默截断在 10000 条，缺数据无人察觉。超限必须显式报错。
	if int64(len(spaces)) < total {
		return nil, nil, fmt.Errorf("空间共 %d 条，超过单次导出上限 10000，已拒绝产出截断文件", total)
	}
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"ID", "名称", "描述", "访问级别", "创建者ID", "创建时间", "更新时间"})
	for _, sp := range spaces {
		_ = w.Write([]string{
			fmt.Sprintf("%d", sp.ID),
			sp.Name,
			sp.Description,
			sp.VisitLevel,
			fmt.Sprintf("%d", sp.CreateUserID),
			formatUnix(sp.CreateTime),
			formatUnix(sp.UpdateTime),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, nil, err
	}
	return []byte(buf.String()), &ExportResult{Module: "spaces", Format: "csv", Filename: "spaces.csv"}, nil
}

// exportSpacesJSON 导出空间列表为 JSON
func (s *ExportService) exportSpacesJSON() ([]byte, *ExportResult, error) {
	spaces, total, err := s.spaceRepo.FindAll(0, 10000, "", "", "")
	if err != nil {
		return nil, nil, err
	}
	if int64(len(spaces)) < total {
		return nil, nil, fmt.Errorf("空间共 %d 条，超过单次导出上限 10000，已拒绝产出截断文件", total)
	}
	type spaceJSON struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Desc       string `json:"description"`
		VisitLevel string `json:"visit_level"`
		CreateUser int    `json:"create_user_id"`
		CreateTime string `json:"create_time"`
		UpdateTime string `json:"update_time"`
	}
	out := make([]spaceJSON, len(spaces))
	for i, sp := range spaces {
		out[i] = spaceJSON{
			ID: sp.ID, Name: sp.Name, Desc: sp.Description,
			VisitLevel: sp.VisitLevel, CreateUser: sp.CreateUserID,
			CreateTime: formatUnix(sp.CreateTime), UpdateTime: formatUnix(sp.UpdateTime),
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return b, &ExportResult{Module: "spaces", Format: "json", Filename: "spaces.json"}, nil
}

// exportUsersCSV 导出用户列表为 CSV
func (s *ExportService) exportUsersCSV() ([]byte, *ExportResult, error) {
	users, total, err := s.userRepo.FindAll(0, 10000, 0, "")
	if err != nil {
		return nil, nil, err
	}
	if int64(len(users)) < total {
		return nil, nil, fmt.Errorf("用户共 %d 条，超过单次导出上限 10000，已拒绝产出截断文件", total)
	}
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"ID", "用户名", "显示名", "邮箱", "角色ID", "禁用", "创建时间"})
	for _, u := range users {
		_ = w.Write([]string{
			fmt.Sprintf("%d", u.ID),
			u.Username,
			u.GivenName,
			u.Email,
			fmt.Sprintf("%d", u.RoleID),
			fmt.Sprintf("%d", u.IsForbidden),
			formatUnix(u.CreateTime),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, nil, err
	}
	return []byte(buf.String()), &ExportResult{Module: "users", Format: "csv", Filename: "users.csv"}, nil
}

// exportUsersJSON 导出用户列表为 JSON
func (s *ExportService) exportUsersJSON() ([]byte, *ExportResult, error) {
	users, total, err := s.userRepo.FindAll(0, 10000, 0, "")
	if err != nil {
		return nil, nil, err
	}
	if int64(len(users)) < total {
		return nil, nil, fmt.Errorf("用户共 %d 条，超过单次导出上限 10000，已拒绝产出截断文件", total)
	}
	type userJSON struct {
		ID          int    `json:"id"`
		Username    string `json:"username"`
		GivenName   string `json:"given_name"`
		Email       string `json:"email"`
		RoleID      int    `json:"role_id"`
		IsForbidden int    `json:"is_forbidden"`
		CreateTime  string `json:"create_time"`
	}
	out := make([]userJSON, len(users))
	for i, u := range users {
		out[i] = userJSON{
			ID: u.ID, Username: u.Username, GivenName: u.GivenName,
			Email: u.Email, RoleID: u.RoleID, IsForbidden: u.IsForbidden,
			CreateTime: formatUnix(u.CreateTime),
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return b, &ExportResult{Module: "users", Format: "json", Filename: "users.json"}, nil
}
