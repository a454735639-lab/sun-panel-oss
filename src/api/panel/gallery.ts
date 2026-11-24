import { post } from '@/utils/request'

export interface GalleryPageRequest {
  page: number
  pageSize: number
  type?: number | null
}

export interface BackupRecoveryRequest {
  fileName: string
  force: boolean
}

/* ---------------- 图库 ---------------- */

export function getUserImagesList<T>(data: GalleryPageRequest) {
  return post<T>({
    url: '/panel/gallery/getUserImagesList',
    data,
  })
}

export function updateGalleryFileType<T>(data: { id: number, type: number }) {
  return post<T>({
    url: '/panel/gallery/updateFileType',
    data,
  })
}

/* ---------------- 公开图库 ---------------- */

export function getPublicImagesList<T>(data: GalleryPageRequest) {
  return post<T>({
    url: '/panel/publicGallery/getImagesList',
    data,
  })
}

export function deletePublicImages<T>(ids: number[]) {
  return post<T>({
    url: '/panel/publicGallery/deletes',
    data: { ids },
  })
}

export function updatePublicGalleryFileType<T>(data: { id: number, type: number }) {
  return post<T>({
    url: '/panel/publicGallery/updateFileType',
    data,
  })
}

export const publicGalleryUploadUrl = '/api/panel/publicGallery/uploadImg'

/* ---------------- 备份 / 迁移 ---------------- */

export function backupCreate<T>() {
  return post<T>({
    url: '/panel/backup/backup',
  })
}

export function backupRecovery<T>(data: BackupRecoveryRequest) {
  return post<T>({
    url: '/panel/backup/recovery',
    data,
  })
}

export const backupUploadZipUrl = '/api/panel/backup/uploadZipFile'
export const backupDownloadUrl = '/api/panel/backup/downloadBackupZIPFile'
