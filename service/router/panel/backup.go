package panel

import (
	"sun-panel/api/api_v1"
	"sun-panel/api/api_v1/middleware"

	"github.com/gin-gonic/gin"
)

func InitBackupRouter(router *gin.RouterGroup) {
	backup := api_v1.ApiGroupApp.ApiPanel.BackupApi

	rAdmin := router.Group("", middleware.LoginInterceptor, middleware.AdminInterceptor)
	{
		rAdmin.POST("/panel/backup/backup", backup.Backup)
		rAdmin.POST("/panel/backup/recovery", backup.Recovery)
		rAdmin.POST("/panel/backup/uploadZipFile", backup.UploadZipFile)
		rAdmin.GET("/panel/backup/downloadBackupZIPFile", backup.DownloadBackupZIPFile)
	}
}
