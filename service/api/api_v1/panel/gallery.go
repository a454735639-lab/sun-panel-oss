package panel

import (
	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/api/api_v1/common/base"
	"sun-panel/models"

	"github.com/gin-gonic/gin"
)

type GalleryApi struct{}

// GetUserImagesList 当前用户的图库/上传图片列表
// POST /panel/gallery/getUserImagesList {"page":1,"pageSize":20,"type":null}
func (a *GalleryApi) GetUserImagesList(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)

	req := struct {
		Page     int  `json:"page"`
		PageSize int  `json:"pageSize"`
		Type     *int `json:"type"`
	}{}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 20
	}

	db := models.Db.Model(&models.File{}).Where("user_id = ?", userInfo.ID)
	if req.Type != nil {
		db = db.Where("type = ?", *req.Type)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}

	var files []models.File
	if err := db.Order("created_at desc").
		Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Find(&files).Error; err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}

	apiReturn.SuccessListData(c, files, count)
}

// UpdateFileType 修改文件类型（如把图库图片设为壁纸）
// POST /panel/gallery/updateFileType {"id":1,"type":1}
func (a *GalleryApi) UpdateFileType(c *gin.Context) {
	userInfo, _ := base.GetCurrentUserInfo(c)

	req := struct {
		ID   uint `json:"id"`
		Type int  `json:"type"`
	}{}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	mFile := models.File{}
	if err := models.Db.Where("id = ? and user_id = ?", req.ID, userInfo.ID).First(&mFile).Error; err != nil {
		apiReturn.ErrorDataNotFound(c)
		return
	}
	if err := models.Db.Model(&mFile).Update("type", req.Type).Error; err != nil {
		apiReturn.ErrorDatabase(c, err.Error())
		return
	}
	apiReturn.SuccessData(c, mFile)
}
