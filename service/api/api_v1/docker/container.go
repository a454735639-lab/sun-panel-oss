package docker

import (
	"context"
	"sun-panel/api/api_v1/common/apiData/dockerApiStructs"
	"sun-panel/api/api_v1/common/apiReturn"
	"sun-panel/biz"
	"sun-panel/lib/cmn"
	"sun-panel/lib/docker"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type Container struct {
}

// SwitchStateByContainerID oss 原生实现：按 ID 启动/停止容器。
// 与 Pro 版 execSwitchActionByContainerID 行为一致，但不做 Pro 授权校验；
// 原 Pro 端点保持原样，购买 Pro 后仍可继续使用。
func (a *Container) SwitchStateByContainerID(c *gin.Context) {
	req := dockerApiStructs.ExecSwitchActionByContainerIDReq{}

	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	client, err := docker.NewDockerClient()
	if err != nil {
		apiReturn.ErrorDockerClientFail(c, err.Error())
		return
	}
	defer client.Close()

	ctx := context.Background()

	if req.Action {
		err = client.ContainerStart(ctx, req.ID, container.StartOptions{})
	} else {
		err = client.ContainerStop(ctx, req.ID, container.StopOptions{})
	}

	if err != nil {
		apiReturn.ErrorDocker(c, err.Error())
		return
	}
	apiReturn.Success(c)
}

func (a *Container) GetList(c *gin.Context) {
	// req := commonApiStructs.RequestPage{}
	// if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
	// 	apiReturn.ErrorParamFomat(c, err.Error())
	// 	return
	// }

	var (
		client docker.Client
		err    error
	)
	client, err = docker.NewClient()
	if err != nil {
		apiReturn.ErrorDockerClientFail(c, err.Error())
		return
	}
	defer client.Close()

	if list, err := client.ListAllContainers(); err != nil {
		apiReturn.ErrorDocker(c, err.Error())
		return
	} else {
		resp := []dockerApiStructs.ContainerInfoResp{}
		for _, v := range list {
			resp = append(resp, dockerApiStructs.ContainerInfoResp{
				ContainerInfo: dockerApiStructs.ContainerInfo{
					ID:         v.ID,
					Name:       v.Names[0][1:],
					Image:      v.Image,
					State:      v.State,
					Status:     v.Status,
					CreateTime: time.Unix(v.Created, 0).Format(cmn.TimeFormatMode1),
					Ports:      a.respPorts(v.Ports),
				},
			})
		}
		apiReturn.SuccessListData(c, resp, 0)
	}
}

func (a *Container) respPorts(ports []types.Port) []dockerApiStructs.ContainerPort {
	respPorts := []dockerApiStructs.ContainerPort{}
	for _, vPort := range ports {
		respPorts = append(respPorts, dockerApiStructs.ContainerPort{
			IP:          vPort.IP,
			PrivatePort: vPort.PrivatePort,
			PublicPort:  vPort.PublicPort,
			Type:        dockerApiStructs.PostType(vPort.Type),
		})
	}
	return respPorts
}

func (a *Container) ExecSwitchActionByContainerID(c *gin.Context) {

	if biz.ProAuth.ProIsExpired() {
		// 没有PRO授权
		apiReturn.ErrorByCode(c, apiReturn.ErrCodeNoPROAuthorization)
		return
	}

	req := dockerApiStructs.ExecSwitchActionByContainerIDReq{}

	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	var (
		client *client.Client
		err    error
	)
	client, err = docker.NewDockerClient()
	if err != nil {
		apiReturn.ErrorDockerClientFail(c, err.Error())
		return
	}
	defer client.Close()

	ctx := context.Background()

	if req.Action {
		startOptions := container.StartOptions{}
		err = client.ContainerStart(ctx, req.ID, startOptions)
	} else {
		startOptions := container.StopOptions{}
		err = client.ContainerStop(ctx, req.ID, startOptions)
	}

	if err != nil {
		apiReturn.ErrorDocker(c, err.Error())
		return
	}
	apiReturn.Success(c)

}

// 获取容器状态
func (a *Container) GetContainerStatus(c *gin.Context) {
	req := dockerApiStructs.ExecSwitchActionByContainerIDReq{}

	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	resp := dockerApiStructs.ContainerStats{}

	inspectData, err := biz.Docker.GetContainerInspect(req.ID)
	if err != nil {
		apiReturn.ErrorDocker(c, err.Error())
		return
	}
	resp.RunStatus = inspectData.Status

	stateData, err := biz.Docker.GetContainerState(req.ID)
	if err != nil {
		apiReturn.ErrorDocker(c, err.Error())
		return
	}
	resp.CPUPercent = stateData.CPUPercent
	resp.Memory = stateData.Memory
	resp.Cache = stateData.Cache
	resp.IORead = stateData.IORead
	resp.IOWrite = stateData.IOWrite
	resp.NetworkRX = stateData.NetworkRX
	resp.NetworkTX = stateData.NetworkTX
	resp.ShotTime = time.Now()

	apiReturn.SuccessData(c, resp)
}

// 获取多个容器的状态
// oss 优化：并发采集 + 5 秒结果缓存。原先串行对每个容器新建 client 做 inspect+stats，
// 20+ 个容器时单次请求要 1.5-2 秒，前端每 5 秒轮询一次会一直占着连接。
func (a *Container) GetContainersState(c *gin.Context) {
	reqIds := []string{}

	if err := c.ShouldBindBodyWith(&reqIds, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	resp := map[string]dockerApiStructs.ContainerStats{}
	now := time.Now()

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, 8) // 并发上限，避免把 docker daemon 打满
	)

	for _, v := range reqIds {
		if cached, ok := containerStatsCache.Load(v); ok {
			if entry, ok := cached.(cachedContainerStats); ok {
				// 有缓存就先返回，过期则后台异步刷新（stale-while-revalidate），
				// 保证前端轮询永远不会卡在 docker daemon 上
				resp[v] = entry.data
				if now.Sub(entry.at) >= containerStatsCacheTTL {
					go refreshContainerStats(v)
				}
				continue
			}
		}

		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			state := collectContainerState(id)
			containerStatsCache.Store(id, cachedContainerStats{data: state, at: time.Now()})

			mu.Lock()
			resp[id] = state
			mu.Unlock()
		}(v)
	}
	wg.Wait()

	apiReturn.SuccessData(c, resp)
}

// refreshContainerStats 后台刷新单个容器的统计（同一容器同时只允许一个刷新任务）
func refreshContainerStats(containerID string) {
	if _, loaded := containerStatsRefreshing.LoadOrStore(containerID, struct{}{}); loaded {
		return
	}
	defer containerStatsRefreshing.Delete(containerID)

	select {
	case containerStateSem <- struct{}{}:
		defer func() { <-containerStateSem }()
	default:
		return // 后台任务过多时丢弃本次刷新，下次轮询再来
	}

	state := collectContainerState(containerID)
	containerStatsCache.Store(containerID, cachedContainerStats{data: state, at: time.Now()})
}

var containerStateSem = make(chan struct{}, 4)

type cachedContainerStats struct {
	data dockerApiStructs.ContainerStats
	at   time.Time
}

var (
	containerStatsCache      sync.Map
	containerStatsRefreshing sync.Map
	containerStatsCacheTTL   = 10 * time.Second
)

// collectContainerState 采集单个容器的运行状态与资源用量
func collectContainerState(containerID string) dockerApiStructs.ContainerStats {
	temp := dockerApiStructs.ContainerStats{}
	inspectData, err := biz.Docker.GetContainerInspect(containerID)
	temp.ShotTime = time.Now()
	if err != nil {
		temp.RunStatus = "error"
		temp.ErrorMassage = err.Error()
		return temp
	}
	temp.RunStatus = inspectData.Status

	if inspectData.Status == "running" {
		stateData, err := biz.Docker.GetContainerState(containerID)
		if err != nil {
			temp.ErrorMassage = err.Error()
			return temp
		}
		temp.CPUPercent = stateData.CPUPercent
		temp.Memory = stateData.Memory
		temp.Cache = stateData.Cache
		temp.IORead = stateData.IORead
		temp.IOWrite = stateData.IOWrite
		temp.NetworkRX = stateData.NetworkRX
		temp.NetworkTX = stateData.NetworkTX
	}
	return temp
}

type ContainerGetContainerIDByNameReq struct {
	Name string `json:"name"`
}

// 根据名称重置容器ID
func (a *Container) GetContainerIDByName(c *gin.Context) {
	req := ContainerGetContainerIDByNameReq{}

	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		apiReturn.ErrorParamFomat(c, err.Error())
		return
	}

	var (
		client docker.Client
		err    error
	)
	client, err = docker.NewClient()
	if err != nil {
		apiReturn.ErrorDockerClientFail(c, err.Error())
		return
	}
	defer client.Close()

	info, err := client.GetContainerByName(req.Name)
	if err != nil {
		apiReturn.ErrorDataNotFound(c)
		return
	}

	apiReturn.SuccessData(c, dockerApiStructs.ContainerInfoResp{
		ContainerInfo: dockerApiStructs.ContainerInfo{
			ID:         info.ID,
			Name:       info.Names[0][1:],
			Image:      info.Image,
			State:      info.State,
			Status:     info.Status,
			CreateTime: time.Unix(info.Created, 0).Format(cmn.TimeFormatMode1),
			Ports:      a.respPorts(info.Ports),
		},
	})
}
