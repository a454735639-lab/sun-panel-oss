package biz

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sun-panel/global"
	"sun-panel/initialize/database"
	"sun-panel/lib/cmn"
	"sun-panel/lib/file"
	"sun-panel/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// ---------- 备份配置（与镜像 v1.8.1 字段一致） ----------

type BackupConfigPath struct {
	Uploads     string `json:"uploads"`
	CustomStyle string `json:"customStyle"`
	Sqlite      string `json:"sqlite"`
}

type BackupConfig struct {
	Name                  string           `json:"name"`
	ConfigVersion         string           `json:"configVersion"`
	SunPanelVersion       string           `json:"sunPanelVersion"`
	SunPanelLowestVersion string           `json:"sunPanelLowestVersion"`
	Time                  string           `json:"time"`
	Paths                 BackupConfigPath `json:"paths"`
}

const (
	BackupConfigVersion          = "1.0"
	BackupDirName                = "runtime/backup" // 备份 zip 落盘目录（相对工作目录）
	BackupTempDir                = "runtime/temp/backup"
	BackupConfigFileName         = "backup.json"
	ErrBackupIncompatibleVersion = "backup version incompatible"
)

// ---------- 版本比较 ----------

// parseVersion 取 "1.8.1-beta25" / "20|1.8.1" 中的数字版本段 [major, minor, patch]
func parseVersion(v string) []int {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, "|"); i >= 0 {
		v = v[i+1:]
	}
	// 截掉首段数字之后的非数字尾巴（如 -beta25）
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	nums := make([]int, 3)
	for j := 0; j < 3 && j < len(parts); j++ {
		nums[j], _ = strconv.Atoi(strings.TrimSpace(parts[j]))
	}
	return nums
}

func versionLess(a, b []int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// VersionCompatibilityCheck 判断备份能否恢复：
//
//	备份版本 <= 当前版本：直接可恢复；
//	备份版本 >  当前版本：若 lowestVersion <= 当前版本，返回 compatible=true 且 newer=true（由调用方决定是否强制）；
//	否则不可恢复。
func VersionCompatibilityCheck(cfg BackupConfig) (compatible, newer bool, err error) {
	cur := parseVersion(cmn.GetSysVersionInfo().Version)
	backup := parseVersion(cfg.SunPanelVersion)
	lowest := parseVersion(cfg.SunPanelLowestVersion)

	switch {
	case !versionLess(cur, backup): // cur >= backup
		return true, false, nil
	case cfg.SunPanelLowestVersion == "" || !versionLess(cur, lowest): // cur >= lowest
		return true, true, nil
	default:
		return false, false, fmt.Errorf("%s: current %s < lowest %s",
			ErrBackupIncompatibleVersion, cmn.GetSysVersionInfo().Version, cfg.SunPanelLowestVersion)
	}
}

// ---------- 备份 ----------

func buildBackupConfig() BackupConfig {
	return BackupConfig{
		Name:                  "sun-panel",
		ConfigVersion:         BackupConfigVersion,
		SunPanelVersion:       cmn.GetSysVersionInfo().Version,
		SunPanelLowestVersion: cmn.GetSysVersionInfo().Version,
		Time:                  cmn.GetTime(),
		Paths: BackupConfigPath{
			Uploads:     "uploads",
			CustomStyle: "custom",
			Sqlite:      "sqlite/database.db",
		},
	}
}

func saveBackupConfig(dir string, cfg BackupConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, BackupConfigFileName), data, 0644)
}

func loadBackupConfig(dir string) (cfg BackupConfig, err error) {
	data, err := os.ReadFile(filepath.Join(dir, BackupConfigFileName))
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &cfg)
	return
}

// backupToTemp 把 uploads / custom / sqlite 汇总到临时目录
func backupToTemp(tmpDir string, cfg BackupConfig) error {
	sourcePath := global.Config.GetValueString("base", "source_path")
	customPath := orDefault(global.Config.GetValueString("base", "custom_style_path"), "./conf/custom")
	sqlitePath := orDefault(global.Config.GetValueString("sqlite", "file_path"), "./conf/database/database.db")

	_ = cfg
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return err
	}
	// 目录可能尚未创建（从未上传过文件/未设置自定义样式），缺失时跳过
	if exists, _ := cmn.PathExists(sourcePath); exists {
		if err := file.CopyDir(sourcePath, filepath.Join(tmpDir, "uploads")); err != nil {
			return fmt.Errorf("copy uploads: %w", err)
		}
	}
	if exists, _ := cmn.PathExists(customPath); exists {
		if err := file.CopyDir(customPath, filepath.Join(tmpDir, "custom")); err != nil {
			return fmt.Errorf("copy custom: %w", err)
		}
	}
	if exists, _ := cmn.PathExists(sqlitePath); exists {
		if err := file.CopyFile(sqlitePath, filepath.Join(tmpDir, "sqlite", "database.db")); err != nil {
			return fmt.Errorf("copy sqlite: %w", err)
		}
	} else {
		return fmt.Errorf("sqlite db not found: %s", sqlitePath)
	}
	return nil
}

// backupPathToZip 把临时目录压成 zip（缺失的子目录自动跳过）
func backupPathToZip(tmpDir, zipPath string) error {
	z := file.ZipFile{}
	items := map[string]string{
		"uploads":            filepath.Join(tmpDir, "uploads"),
		"sqlite":             filepath.Join(tmpDir, "sqlite"),
		BackupConfigFileName: filepath.Join(tmpDir, BackupConfigFileName),
	}
	if exists, _ := cmn.PathExists(filepath.Join(tmpDir, "custom")); exists {
		items["custom"] = filepath.Join(tmpDir, "custom")
	}
	return z.Create(items, zipPath)
}

// BackupToZip 入口：备份并打包，返回文件名（不含路径）
func BackupToZip() (fileName string, err error) {
	cfg := buildBackupConfig()
	ts := time.Now().Format("20060102150405")
	fileName = fmt.Sprintf("sun-panel-backup-%s.zip", ts)

	tmpDir := filepath.Join(BackupTempDir, ts)
	defer os.RemoveAll(tmpDir)

	if err = backupToTemp(tmpDir, cfg); err != nil {
		return
	}
	if err = saveBackupConfig(tmpDir, cfg); err != nil {
		return
	}
	zipPath := filepath.Join(BackupDirName, fileName)
	if err = os.MkdirAll(BackupDirName, os.ModePerm); err != nil {
		return
	}
	if err = backupPathToZip(tmpDir, zipPath); err != nil {
		return
	}
	return
}

// ---------- 恢复 ----------

// ExtractBackZip 解压上传的备份包
func ExtractBackZip(zipPath, destDir string) error {
	z := file.ZipFile{}
	return z.Extract(zipPath, destDir)
}

// ExtractBackupZipAndLoadConfig 解压并读取 backup.json
func ExtractBackupZipAndLoadConfig(zipPath string) (cfg BackupConfig, dir string, err error) {
	dir = filepath.Join(BackupTempDir, "restore-"+time.Now().Format("20060102150405"))
	if err = ExtractBackZip(zipPath, dir); err != nil {
		return
	}
	cfg, err = loadBackupConfig(dir)
	return
}

// deleteSqliteDatabaseImportantData 恢复场景保留：清空敏感表。
// 当前恢复实现采用整库文件替换，仅在 mysql 等无法整库替换的场景下由调用方使用。
func deleteSqliteDatabaseImportantData(db *gorm.DB) error {
	for _, table := range []string{"open_api_token", "user_config", "file"} {
		if err := db.Exec(fmt.Sprintf("DELETE FROM `%s`", table)).Error; err != nil {
			return err
		}
	}
	return nil
}

// reopenSqlite 关闭当前连接并用原配置重新打开 sqlite
func reopenSqlite(filePath string) error {
	if sqlDB, err := global.Db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	db, err := gorm.Open(sqlite.Open(filePath), &gorm.Config{
		Logger:                                   database.GetLogger(),
		NamingStrategy:                           schema.NamingStrategy{SingularTable: true},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return err
	}
	// 同时更新两处全局句柄，避免残留已关闭的旧连接
	global.Db = db
	models.Db = db
	return nil
}

// Recovery 恢复备份：sqlite 整库替换 + 还原 uploads / custom。
// force=false 且备份版本比当前新（但兼容）时返回 lowestSunPanelVersion 供前端确认。
func Recovery(zipPath string, force bool) (needConfirm bool, lowestSunPanelVersion string, err error) {
	cfg, dir, err := ExtractBackupZipAndLoadConfig(zipPath)
	if err != nil {
		return
	}
	defer os.RemoveAll(dir)

	compatible, newer, cerr := VersionCompatibilityCheck(cfg)
	if cerr != nil {
		return false, cfg.SunPanelLowestVersion, cerr
	}
	if !compatible {
		return false, cfg.SunPanelLowestVersion, fmt.Errorf("%s", ErrBackupIncompatibleVersion)
	}
	if newer && !force {
		return true, cfg.SunPanelLowestVersion, nil
	}

	// 1) sqlite 整库替换
	if strings.EqualFold(orDefault(global.Config.GetValueString("base", "database_drive"), "sqlite"), "sqlite") {
		sqlitePath := orDefault(global.Config.GetValueString("sqlite", "file_path"), "./conf/database/database.db")
		if err = reopenSqlite(sqlitePath); err != nil {
			return
		}
		if err = file.CopyFile(filepath.Join(dir, cfg.Paths.Sqlite), sqlitePath); err != nil {
			return
		}
		if err = reopenSqlite(sqlitePath); err != nil {
			return
		}
	}

	// 2) 还原 uploads（备份包里可能没有该目录）
	sourcePath := global.Config.GetValueString("base", "source_path")
	if exists, _ := cmn.PathExists(filepath.Join(dir, cfg.Paths.Uploads)); exists {
		if err = file.CopyDir(filepath.Join(dir, cfg.Paths.Uploads), sourcePath); err != nil {
			return
		}
	}

	// 3) 还原 custom（同上，可能缺失）
	customPath := orDefault(global.Config.GetValueString("base", "custom_style_path"), "./conf/custom")
	if exists, _ := cmn.PathExists(filepath.Join(dir, cfg.Paths.CustomStyle)); exists {
		err = file.CopyDir(filepath.Join(dir, cfg.Paths.CustomStyle), customPath)
	}
	return
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
