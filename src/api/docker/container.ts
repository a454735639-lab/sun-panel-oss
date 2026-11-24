import { post } from '@/utils/request'

export function getList<T>() {
  return post<T>({
    url: '/docker/container/getList',
  })
}

export function execSwitchAction<T>(param: Docker.Container.ExecSwitchActionReq) {
  // oss 原生端点，与 Pro 端点行为一致（后端 /docker/container/switchState）
  return post<T>({
    url: '/docker/container/switchState',
    data: param,
  })
}

export function getContainerStatus<T>(containerId: string) {
  return post<T>({
    url: '/docker/container/getContainerStatus',
    data: { id: containerId },
  })
}

export function getContainersStats<T>(containerIds: string[]) {
  return post<T>({
    url: '/docker/container/getContainersStats',
    data: containerIds,
  })
}
export function getContainerIDByName(name: string) {
  return post<Docker.Container.Info>({
    url: '/docker/container/getContainerIDByName',
    data: { name },
  })
}
