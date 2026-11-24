# Sun-Panel 1.8.1-oss · NAS 上构建部署指南

> 目标：在你的 NAS（arm64 Linux）上把「开源 master + 图库/公开图库/备份三模块」构建成 docker 镜像并部署。
> 补丁已通过 `git apply --check` 校验，21 个文件，+1513/-8 行。

## 0. 前置条件

- NAS 上有 docker（含 buildx 或普通 build）
- 能访问 GitHub（或用镜像加速）
- 磁盘 ≥ 3GB（node_modules + go 编译缓存）

## 1. 获取源码并打补丁

```bash
mkdir -p /volume1/docker/sun-panel-build && cd /volume1/docker/sun-panel-build

# 方式一：git（推荐）
git clone https://github.com/hslr-s/sun-panel.git src && cd src
git apply --check /path/to/gallery-backup-modules.patch   # 先校验
git apply /path/to/gallery-backup-modules.patch

# 方式二：tarball
curl -L -o master.tgz https://codeload.github.com/hslr-s/sun-panel/tar.gz/refs/heads/master
mkdir src && tar -xzf master.tgz -C src --strip-components=1
cd src && patch -p1 < /path/to/gallery-backup-modules.patch
```

> 补丁里的版本串是 `20|1.8.1-oss`；构建容器内会重新生成 `assets/bindata.go`，无需手动处理。

## 2. 构建镜像

```bash
cd /volume1/docker/sun-panel-build/src
docker build -t sun-panel:1.8.1-oss .
# NAS 是 arm64 时直接 build 即可；x86 NAS 构建给 arm64 设备用：
# docker buildx build --platform linux/arm64 -t sun-panel:1.8.1-oss --load .
```

预计 8-15 分钟（前端 pnpm install + build，后端 go build）。

## 3. 部署

```bash
cd /volume1/docker/sun-panel-build
docker compose up -d
```

`docker-compose.yml` 已随本指南放在同级目录，挂载：

| 容器内 | 宿主机 | 说明 |
|---|---|---|
| /app/conf | ./conf | 配置 + sqlite 数据库 + uploads |
| /app/runtime | ./runtime | 备份 zip、临时文件 |
| /var/run/docker.sock（可选） | 同名 | Docker 卡片需要 |

数据迁移：直接把老实例的 `conf/` 目录整个拷过来（sqlite 库 + uploads 都在里面），新库自动补 `is_public_gallery` 列。

## 4. 验收清单

- [ ] 登录后设置菜单出现：图库 / 公开图库 / 迁移
- [ ] 公开图库上传图片 → 列表出现
- [ ] 迁移页「立即备份」→ 自动下载 zip
- [ ] 上传 zip → 恢复 → 提示成功 → 重新登录
- [ ] `docker exec <容器> ./sun-panel -h` 显示 `Version: 1.8.1-oss`

## 5. 与原版镜像的差异（已知）

- 后端由开源 master 补齐三模块，函数级行为按镜像 v1.8.1 的 DWARF 规格实现；响应字段实测对齐（`{count,list}` 分页、`isPublicGallery` 等）。
- 原版 Pro 授权模块为闭源功能，开源基座带 `ProAuth` 页面但未购买 Pro 时功能不变。
- 前端三页面为重新实现（原版前端源码不可得），交互对齐原版入口与文案。

## 6. 回滚

```bash
cd /volume1/docker/sun-panel-build/src && git apply -R gallery-backup-modules.patch  # 撤补丁
docker compose down && docker run -d -p 3002:3002 -v ./conf:/app/conf hslr/sun-panel:latest  # 回原版镜像
```

conf 目录不变，原版镜像可直接接管数据（新增列 `is_public_gallery` 对原版无影响，只是多一个空列）。
