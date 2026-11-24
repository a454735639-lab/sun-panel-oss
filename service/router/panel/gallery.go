package panel

import (
	"sun-panel/api/api_v1"
	"sun-panel/api/api_v1/middleware"

	"github.com/gin-gonic/gin"
)

func InitGallery(router *gin.RouterGroup) {
	gallery := api_v1.ApiGroupApp.ApiPanel.GalleryApi
	r := router.Group("", middleware.LoginInterceptor)
	{
		r.POST("/panel/gallery/getUserImagesList", gallery.GetUserImagesList)
		r.POST("/panel/gallery/updateFileType", gallery.UpdateFileType)
	}
}
