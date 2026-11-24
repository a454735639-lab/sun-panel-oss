# 缺失模块实现规格：图库 / 公开图库 / 备份恢复

> 目标：在开源 master 基座上补回镜像独有的 3 个后端模块，API 与数据模型与镜像 v1.8.1 保持一致。
> 依据：DWARF 符号（函数名+行号）、`models.File` 结构、sqlite `file` 表、前端反混淆产物中的请求封装。

## 0. 基座构建状态（2026-11-24 已完成）

| 项 | 状态 |
|---|---|
| 后端编译 | ✅ `service/sun-panel-build`（25.1 MB，sha256 `6849712e8ecbe626…`），实跑通过 |
| 前端编译 | ✅ `npm install`（1081 包，用 `--legacy-peer-deps`，package-lock 与 package.json 不同步，`npm ci` 会拒绝）→ `npm run build` → `dist/` |
| 冒烟 | ✅ 整合后 `/` 200、`/user/getInfo` 200；`gallery/backup` 路由按预期 404（未实现） |
| 版本串 | 显示 `1.7.0-beta25-03-20`：来自 `service/assets/version`（`20|1.7.0-beta25-03-20`），改成 `20|1.8.1-oss` 后需重新 `go-bindata-assetfs` + `go build` |
| 构建注意 | iSH 下 sqlite cgo 必须 `CGO_CFLAGS="-D_LARGEFILE64_SOURCE"`（**不要带 -O2**，否则单个 sqlite3-binding.c 编译 >20 min） |

一键复现：

```bash
cd /var/minis/shared/sun-panel-re/src/sun-panel-master
# 前端
npm install --no-audit --no-fund --legacy-peer-deps
npm run build                       # iSH 上约 9 分钟
# 后端
cd service
export PATH=$PATH:$(go env GOPATH)/bin CGO_ENABLED=1 CGO_CFLAGS="-D_LARGEFILE64_SOURCE" GOTOOLCHAIN=local
go-bindata-assetfs -o=assets/bindata.go -pkg=assets assets/...
go build -o sun-panel-build --ldflags="-X sun-panel/global.RUNCODE=release -X sun-panel/global.ISDOCKER=true" main.go
# 整合运行
mkdir -p /tmp/mtest/conf && cp -r ../dist /tmp/mtest/web && cp sun-panel-build /tmp/mtest/
cd /tmp/mtest && ./sun-panel-build -config-path ./conf
```


## 1. 现状对齐

| 项 | 镜像 v1.8.1 | 开源 master | 处理 |
|---|---|---|---|
| `models/file.go` | 有，含 `IsPublicGallery bool` | 有，**缺该字段** | 加字段 + AutoMigrate |
| `file` 表 | 9 列，含 `is_public_gallery numeric` | 8 列 | 迁移后自动补列 |
| 文件上传 `/api/file/uploadImg` | 有 | 有 | 复用 |
| 图库/备份 handler | 有 | **无** | 按下表实现 |

## 2. 数据模型增量

```go
// models/file.go 追加
IsPublicGallery bool `json:"isPublicGallery" gorm:"default:false"` // 是否公开到公共图库

// Type 取值（镜像实测）
const (
    FileTypeWallpaper  = 1 // 壁纸
    FileTypeCardIcon   = 2 // 卡片图标
    FileTypeGalleryImg = 3 // 图库图片（新增）
)
```

`file` 表（镜像实测 schema）：

```sql
CREATE TABLE `file` (id, created_at, updated_at, deleted_at,
  src TEXT, user_id INTEGER, file_name TEXT, type INTEGER, ext TEXT, is_public_gallery NUMERIC);
```

## 3. 接口清单（8 个端点，全部已在前端产物中确证）

| # | 方法 | 路径 | 鉴权 | 请求 | 响应 `data` |
|---|---|---|---|---|---|
| 1 | POST | `/panel/gallery/getUserImagesList` | 登录 | `{page, pageSize, type?}` | `{list: []File, total}` |
| 2 | POST | `/panel/gallery/updateFileType` | 登录 | `{id:uint, type:int}` | `File` |
| 3 | POST | `/panel/publicGallery/getImagesList` | 登录/公开 | `{page, pageSize}` | `{list: []File, total}` |
| 4 | POST | `/panel/publicGallery/deletes` | 登录 | `{ids: []uint}` | `null` |
| 5 | POST | `/api/panel/publicGallery/uploadImg` | 登录 | multipart `file` | `File`（isPublicGallery=true） |
| 6 | POST | `/panel/backup/backup` | **管理员** | 无 | `{fileName}` |
| 7 | POST | `/panel/backup/recovery` | **管理员** | `{fileName, force:bool}` | `null` |
| 8 | POST | `/api/panel/backup/uploadZipFile` | **管理员** | multipart `file`(zip) | `null` |
| 9 | GET | `/api/panel/backup/downloadBackupZIPFile` | **管理员** | query `fileName` | zip 流 |

响应包体沿用项目统一格式（`apiReturn.Success/Error`）：`{"code":0,"data":...,"msg":""}`。

### 3.1 实测响应样本（2026-11-24 从原版 v1.8.1 实例抓取，Observed）

```jsonc
// POST /panel/gallery/getUserImagesList {"page":1,"pageSize":20}
{"code":0,"data":{"count":0,"list":[]},"msg":"OK"}
// POST /panel/publicGallery/getImagesList {"page":1,"pageSize":20}
{"code":0,"data":{"count":0,"list":[]},"msg":"OK"}
// POST /panel/proAuth/getStatus
{"code":0,"data":{"proExpiration":null,"loginStatus":false,"username":"","nickName":""},"msg":"OK"}
// POST /about/version/runningInfo
{"code":0,"data":{"configPath":"./conf","webPath":"./web","customStylePath":"./conf/custom",
  "sourcePath":"./conf/uploads","sourceTempPath":"./runtime/temp","binaryFilePath":"/app/sun-panel"},"msg":"OK"}
```

分页格式为 `{count, list}`（不是 gorm 默认的 total/rows）——实现时对齐。
File 列表项字段（Inferred，由 models.File + 实例 user 对象风格推断）：
`{ID, CreatedAt, UpdatedAt, DeletedAt, src, userId, fileName, type, ext, isPublicGallery}`。

### 3.2 备份页实际交互（修正第 3 节）

- **没有列表接口**：备份页只做「备份/上传恢复/下载」，不展示历史列表；
- `POST /panel/backup/backup` → 响应 `data.fileName`，前端拿到后**立即触发下载**；
- 下载：`GET /api/panel/backup/downloadBackupZIPFile?fileName=xxx`（fetch + token 头 + blob）；
- 上传 zip：`POST /api/panel/backup/uploadZipFile` → 响应 `data.fileName` → 前端自动调用 recovery；
- recovery 兼容性失败时，错误回调里带 `lowestSunPanelVersion`，前端弹「版本较新但兼容，是否强制」确认后以 `force:true` 重试。

## 4. 备份模块设计（对齐镜像 `biz/backup.go`）

镜像实测结构（DWARF）：

```go
type BackupConfigPath struct { Uploads, CustomStyle, Sqlite string }
type BackupConfig struct {
    Name                   string           // 备份名
    ConfigVersion          string           // 备份格式版本
    SunPanelVersion        string           // 生成备份的程序版本
    SunPanelLowestVersion  string           // 可恢复的最低版本
    Time                   string
    Paths                  BackupConfigPath
}
type BackupType struct { CurrentConfigVersion, CurrentSunPanelLowestVersion string }
```

镜像里的函数序列（`biz/backup.go`，行号来自 DWARF，可作为实现顺序）：

| 行 | 函数 | 作用 |
|---|---|---|
| 66 | `buildConfig` | 组装 BackupConfig |
| 78 | `backupToTemp` | 把 uploads / custom / sqlite 复制到临时目录 |
| 135 | `backupPathToZip` | 目录压 zip |
| 161 | `BackupToZip` | 入口：临时目录 → zip → 落盘 |
| 176 | `saveBackupConfig` | 写 backup.json |
| 189 | `loadBackupConfig` | 读 backup.json |
| 212 | `ExtractBackZip` | 解压上传的 zip |
| 234 | `ExtractBackupZipAndLoadConfig` | 解压 + 读配置 |
| 248 | `Recovery` | 恢复：清库 → 还原文件 → 重载 |
| 327 | `deleteSqliteDatabaseImportantData` | 恢复前清空敏感表（user/open_api_token 等） |
| 358 | `VersionCompatibilityCheck` | 版本兼容性判定 |

**zip 布局**（由 Paths 字段推断，实现时以 `backup.json` 为准）：

```
backup.zip
├── backup.json          # BackupConfig
├── uploads/             # source_path 下的上传文件
├── custom/              # custom_style_path
└── sqlite/database.db   # 仅 sqlite 驱动时
```

**关键行为**：
- 备份文件落 `./runtime/backup/`，列表接口直接读目录（前端 `href` 拼接下载）。
- `force=false` 时若检测到版本不兼容直接拒绝；`force=true` 跳过版本检查。
- 恢复必须先关闭当前 sqlite 连接（`gorm` 关闭 + 重开），否则文件被占用。

## 5. 文件级 TODO（对应镜像的 11 个文件）

| 镜像文件 | 建议落地 | 说明 |
|---|---|---|
| `api/api_v1/panel/gallery.go` | 新建 | `GetUserImagesList`(L15)、`UpdateFileType`(L83) |
| `api/api_v1/panel/publicGallery.go` | 新建 | `GetImagesList`(L21)、`UploadImg`(L88)、`Deletes`(L139)、`UpdateFileType`(L199) |
| `api/api_v1/panel/backup.go` | 新建 | `Backup`(L18)、`DownloadBackupZIPFile`(L34)、`UploadZipFile`(L156)、`Recovery`(L197) |
| `router/panel/gallery.go` | 新建 | `InitGallery`(L10)，在 `router/panel/A_ENTER.go` 注册 |
| `router/panel/publicGallery.go` | 新建 | `InitPublicGallery`(L10) |
| `router/panel/backup.go` | 新建 | `InitBackup`(L10)，走 AdminInterceptor |
| `biz/backup.go` | 新建 | 见第 4 节函数表 |
| `biz/config.go` | 新建 | 配置导出/导入辅助 |
| `lib/file/base.go` | 新建 | `CopyDir`(L6)、`CopyFile`(L46) |
| `lib/file/zip.go` | 新建 | `ZipFile.Create`(L16)、`Extract`(L82) |
| `assets/bindata.go` | **无需补** | master 已改为运行时读 `./web`，编译前由 `go-bindata-assetfs` 生成 |

## 6. 前端对接（无需改前端即可验证）

### 6.1 菜单/页面差异（原版 v1.8.1 vs master 基座，实测）

原版设置菜单共 12 项（从镜像前端路由配置提取）：

| # | 菜单 | componentName | 镜像 | master |
|---|---|---|---|---|
| 1 | 用户信息 | UserInfo | ✅ | ✅ |
| 2 | 基础设置 | Style | ✅ | ✅ |
| 3 | 分组管理 | ItemGroupManage | ✅ | ✅ |
| 4 | **图库** | **Gallery** | ✅ | ❌ 缺 |
| 5 | 导入导出 | ImportExport | ✅ | ✅ |
| 6 | Open API | OpenApi | ✅ | ✅（DockerManage 之外另有 UploadFileManager 目录，菜单未注册） |
| 7 | 用户管理 | Users（管理员） | ✅ | ✅ |
| 8 | **公开图库** | **PublicGallery**（管理员） | ✅ | ❌ 缺 |
| 9 | 全局设置 | GlobalSetting（管理员） | ✅ | ✅ |
| 10 | Pro 授权 | ProAuth（管理员） | ✅ | ✅ |
| 11 | **迁移** | **BackupMigration**（管理员） | ✅ | ❌ 缺 |
| 12 | 关于 | About | ✅ | ✅ |

即 **master 前端缺 3 个页面**：`Gallery`、`PublicGallery`、`BackupMigration`（对应 i18n key：`common.gallery` / `common.publicGallery` / `apps.backupMigration.*`）。

### 6.2 页面 ↔ 接口对应

| 页面 | chunk | 调用的接口 |
|---|---|---|
| 图库 | `index-B9frhAwk.js`（内含 `getUserImagesList`/`updateFileType` 封装） | 1、2 |
| 公开图库 | `index-DJUoBrTO.js`（上传组件 `upload-url: /api/panel/publicGallery/uploadImg`） | 3、4、5 |
| 备份 | `index-27MHNKMR.js`（`function I(){return _({url:"/panel/backup/backup"})}`） | 6、7、8、9 |

若直接用开源前端（`src/`），这三个页面在开源版已存在，只是后端接口 404。

## 7. 验收方式（可与镜像实例对照）

```bash
# 同一请求分别打「原镜像实例 :3002」和「自编译实例 :3012」，比较 code/data 结构
curl -s -X POST http://127.0.0.1:3002/panel/gallery/getUserImagesList -H 'Content-Type: application/json' -d '{"page":1,"pageSize":20}'
curl -s -X POST http://127.0.0.1:3012/panel/gallery/getUserImagesList -H 'Content-Type: application/json' -d '{"page":1,"pageSize":20}'
```

比对项：字段名（`src/fileName/type/ext/isPublicGallery`）、分页结构、错误码。
