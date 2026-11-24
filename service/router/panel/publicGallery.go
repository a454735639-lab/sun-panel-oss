package panel

import (
	"sun-panel/api/api_v1"
	"sun-panel/api/api_v1/middleware"

	"github.com/gin-gonic/gin"
)

func InitPublicGallery(router *gin.RouterGroup) {
	publicGallery := api_v1.ApiGroupApp.ApiPanel.PublicGalleryApi

	// 列表登录即可见；上传/删除/改类型仅管理员
	r := router.Group("", middleware.LoginInterceptor)
	{
		r.POST("/panel/publicGallery/getImagesList", publicGallery.GetImagesList)
	}

	rAdmin := router.Group("", middleware.LoginInterceptor, middleware.AdminInterceptor)
	{
		rAdmin.POST("/panel/publicGallery/uploadImg", publicGallery.UploadImg)
		rAdmin.POST("/panel/publicGallery/deletes", publicGallery.Deletes)
		rAdmin.POST("/panel/publicGallery/updateFileType", publicGallery.UpdateFileType)
	}
}
