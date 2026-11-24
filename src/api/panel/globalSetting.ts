import { post } from '@/utils/request'

export function saveSiteStting<T>(req: GlobalSetting.SiteSetting) {
  return post<T>({
    url: '/panel/globalSetting/saveSiteStting',
    data: req,
  })
}

export function getSiteStting<T>() {
  return post<T>({
    url: '/panel/globalSetting/getSiteStting',
  })
}

export function getCustomStylePath<T>() {
  return post<T>({
    url: '/panel/globalSetting/getCustomStylePath',
  })
}

export function getCustomJsAndCssCode<T>() {
  return post<T>({
    url: '/panel/globalSetting/getCustomJsAndCssCode',
  })
}

export function saveCustomJsAndCssCode<T>(data: GlobalSetting.GlobalSettingCustomJsAndCss) {
  // oss 原生端点，与 Pro 端点行为一致（后端 /panel/globalSetting/saveCustomStyleCodeOss）
  return post<T>({
    url: '/panel/globalSetting/saveCustomStyleCodeOss',
    data,
  })
}
