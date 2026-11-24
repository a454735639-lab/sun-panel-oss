package panel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/biz"
	"sun-panel/global"

	"github.com/gin-gonic/gin"
)

type BackupApi struct{}

var backupAgreeExts = []string{".zip"}

func backupSavePath(fileName string) string {
	return filepath.Join(biz.BackupDirName, filepath.Base(fileName))
}

// Backup 执行备份并返回文件名
// POST /panel/backup/backup
func (a *BackupApi) Backup(c *gin.Context) {
	fileName, err := biz.BackupToZip()
	if err != nil {
		global.Logger.Error(err.Error())
		apiReturn.Error(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, gin.H{"fileName": fileName})
}

// DownloadBackupZIPFile 下载备份包
// GET /api/panel/backup/downloadBackupZIPFile?fileName=xxx.zip
func (a *BackupApi) DownloadBackupZIPFile(c *gin.Context) {
	fileName := c.Query("fileName")
	if fileName == "" || strings.Contains(fileName, "..") || strings.Contains(fileName, "/") {
		apiReturn.ErrorParamFomat(c, "illegal fileName")
		return
	}
	zipPath := backupSavePath(fileName)
	if _, err := os.Stat(zipPath); err != nil {
		apiReturn.ErrorDataNotFound(c)
		return
	}
	c.FileAttachment(zipPath, fileName)
}

// UploadZipFile 上传备份包（不自动恢复）
// POST /api/panel/backup/uploadZipFile  multipart: imgfile
func (a *BackupApi) UploadZipFile(c *gin.Context) {
	f, err := c.FormFile("imgfile")
	if err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}
	if !containsExt(backupAgreeExts, f.Filename) {
		apiReturn.ErrorByCode(c, apiReturn.ErrCodeUnsupportedFileFormat)
		return
	}

	if err := os.MkdirAll(biz.BackupDirName, os.ModePerm); err != nil {
		apiReturn.Error(c, err.Error())
		return
	}
	fileName := fmt.Sprintf("upload-%s.zip", time.Now().Format("20060102150405"))
	if err := c.SaveUploadedFile(f, backupSavePath(fileName)); err != nil {
		apiReturn.Error(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, gin.H{"fileName": fileName})
}

// Recovery 恢复备份
// POST /panel/backup/recovery {"fileName":"xxx.zip","force":false}
func (a *BackupApi) Recovery(c *gin.Context) {
	req := struct {
		FileName string `json:"fileName"`
		Force    bool   `json:"force"`
	}{}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}
	if req.FileName == "" || strings.Contains(req.FileName, "..") || strings.Contains(req.FileName, "/") {
		apiReturn.ErrorParamFomat(c, "illegal fileName")
		return
	}

	needConfirm, lowest, err := biz.Recovery(backupSavePath(req.FileName), req.Force)
	if err != nil {
		global.Logger.Error(err.Error())
		// 版本不兼容：data 里带 lowestSunPanelVersion，前端弹确认框
		apiReturn.ErrorCode(c, 1002, err.Error(), gin.H{"lowestSunPanelVersion": lowest})
		return
	}
	if needConfirm && !req.Force {
		apiReturn.ErrorCode(c, 1002, "recoveryFileVersionNewerButCompatible",
			gin.H{"lowestSunPanelVersion": lowest})
		return
	}
	apiReturn.SuccessData(c, gin.H{"fileName": req.FileName})
}

func containsExt(exts []string, name string) bool {
	lower := strings.ToLower(name)
	for _, e := range exts {
		if strings.HasSuffix(lower, e) {
			return true
		}
	}
	return false
}
