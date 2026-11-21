package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
)

// BackupAPI 备份与恢复管理 API
type BackupAPI struct {
	backupService *adminsvc.BackupService
}

// NewBackupAPI 创建备份 API 实例
func NewBackupAPI() *BackupAPI {
	return &BackupAPI{
		backupService: adminsvc.NewBackupService(),
	}
}

// CreateBackup 创建备份
// @Summary 创建备份
// @Tags 备份管理
// @Produce json
// @Success 200
// @Router /api/v1/backups [post]
func (api *BackupAPI) CreateBackup(c *gin.Context) {
	info, err := api.backupService.CreateBackup()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.SuccessWithMsg(c, info, "备份创建成功")
}

// ListBackups 列出所有备份
// @Summary 备份列表
// @Tags 备份管理
// @Produce json
// @Success 200
// @Router /api/v1/backups [get]
func (api *BackupAPI) ListBackups(c *gin.Context) {
	backups, err := api.backupService.ListBackups()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"backups": backups, "total": len(backups)})
}

// DownloadBackup 下载备份文件
// @Summary 下载备份
// @Tags 备份管理
// @Produce application/octet-stream
// @Param id path string true "备份 ID"
// @Router /api/v1/backups/{id}/download [get]
func (api *BackupAPI) DownloadBackup(c *gin.Context) {
	id := c.Param("id")
	path, err := api.backupService.GetBackupFilePath(id)
	if err != nil {
		response.FailSafe(c, http.StatusNotFound, "备份不存在", err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=\""+id+".zip\"")
	c.Header("Content-Type", "application/zip")
	c.File(path)
}

// ImportBackup 导入外部备份包
// @Summary 导入备份包
// @Description 上传备份 zip（须含根下 database.sql），落到备份列表后即可恢复
// @Tags 备份管理
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "备份包 (.zip)"
// @Success 200
// @Router /api/v1/backups/import [post]
func (api *BackupAPI) ImportBackup(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequestKeyDetail(c, response.MsgInvalidParameter, "未收到上传文件")
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	defer f.Close()

	info, err := api.backupService.ImportBackup(fileHeader.Filename, f)
	if err != nil {
		// 校验类失败（空包 / 不是 zip / 缺 database.sql）都是请求内容问题，回 400 并带上原因
		response.BadRequestKeyDetail(c, response.MsgInvalidParameter, err.Error())
		return
	}
	response.SuccessWithMsg(c, info, "备份导入成功")
}

// DeleteBackup 删除备份
// @Summary 删除备份
// @Tags 备份管理
// @Param id path string true "备份 ID"
// @Produce json
// @Success 200
// @Router /api/v1/backups/{id} [delete]
func (api *BackupAPI) DeleteBackup(c *gin.Context) {
	id := c.Param("id")
	if err := api.backupService.DeleteBackup(id); err != nil {
		response.FailSafe(c, http.StatusBadRequest, "删除备份失败", err)
		return
	}
	response.SuccessWithMsg(c, nil, "备份删除成功")
}

// RestoreBackup 从备份恢复
// @Summary 从备份恢复
// @Tags 备份管理
// @Param id path string true "备份 ID"
// @Produce json
// @Success 200
// @Router /api/v1/backups/{id}/restore [post]
func (api *BackupAPI) RestoreBackup(c *gin.Context) {
	id := c.Param("id")
	// 二次确认：要求 query 参数 confirm=1
	confirm := c.DefaultQuery("confirm", "0")
	if confirm != "1" {
		response.BadRequestKey(c, response.MsgRestoreNeedsConfirm)
		return
	}
	if err := api.backupService.RestoreBackup(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.SuccessWithMsg(c, nil, "恢复成功")
}
