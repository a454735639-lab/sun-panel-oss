<script setup lang="ts">
import { NAlert, NButton, NUpload, useDialog, useMessage } from 'naive-ui'
import type { UploadFileInfo } from 'naive-ui'
import { ref } from 'vue'
import { backupCreate, backupDownloadUrl, backupRecovery, backupUploadZipUrl } from '@/api/panel/gallery'
import { SvgIcon } from '@/components/common'
import { t } from '@/locales'
import { useAuthStore } from '@/store'

const ms = useMessage()
const dialog = useDialog()
const authStore = useAuthStore()
const backing = ref(false)
const restoring = ref(false)

function downloadBackup(fileName: string) {
  fetch(`${backupDownloadUrl}?fileName=${encodeURIComponent(fileName)}`, {
    headers: { token: authStore.token || '' },
  }).then(res => res.blob()).then((blob) => {
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = fileName
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    window.URL.revokeObjectURL(url)
  })
}

function handleBackup() {
  backing.value = true
  backupCreate<{ fileName: string }>().then((res) => {
    if (res.data.fileName) {
      ms.success(t('apps.backupMigration.backupSuccess'))
      downloadBackup(res.data.fileName)
    }
  }).catch(e => ms.error(e?.msg || t('apps.backupMigration.backupFailed'))).finally(() => {
    backing.value = false
  })
}

function doRecovery(fileName: string, force: boolean) {
  restoring.value = true
  backupRecovery<{ fileName: string }>({ fileName, force }).then(() => {
    ms.success(t('apps.backupMigration.restoreSuccess'), { duration: 10000 })
    window.location.href = '/#/clear'
    window.location.reload()
  }).catch((e: any) => {
    // 版本较新但兼容：弹确认后强制恢复
    if (e?.data?.lowestSunPanelVersion) {
      dialog.warning({
        title: t('common.warning'),
        content: t('apps.backupMigration.restoreVersionConfirm', {
          backupVersion: e.data.lowestSunPanelVersion,
          lowestSunPanelVersion: e.data.lowestSunPanelVersion,
        }),
        positiveText: t('common.continue'),
        negativeText: t('common.cancel'),
        onPositiveClick: () => {
          restoring.value = true
          doRecovery(fileName, true)
        },
      })
    }
    else {
      ms.error(`${t('apps.backupMigration.restoreFailed')}: ${e?.msg || e}`)
    }
  }).finally(() => {
    restoring.value = false
  })
}

function handleUploadFinish({ event }: { event?: ProgressEvent }) {
  const target = (event as any)?.target
  if (!target?.response)
    return
  try {
    const res = JSON.parse(target.response)
    if (res.code === 0 && res.data.fileName)
      doRecovery(res.data.fileName, false)
    else
      ms.error(res.msg || t('apps.backupMigration.restoreFailed'))
  }
  catch {
    ms.error(t('apps.backupMigration.restoreFailed'))
  }
}

function beforeUpload({ file }: { file: UploadFileInfo }) {
  restoring.value = true
  if (!file.name?.toLowerCase().endsWith('.zip')) {
    ms.error(t('apps.backupMigration.restoreFailed'))
    restoring.value = false
    return false
  }
  return true
}
</script>

<template>
  <div class="p-2 h-full">
    <NAlert type="warning" :bordered="false">
      {{ $t('apps.backupMigration.tip') }}
    </NAlert>

    <div class="flex flex-col items-center gap-[30px] m-[40px]">
      <NButton type="info" size="large" :loading="backing" @click="handleBackup">
        <template #icon>
          <SvgIcon icon="fa6:solid-file-zipper" />
        </template>
        {{ $t('apps.backupMigration.backup') }}
      </NButton>

      <NUpload
        :action="backupUploadZipUrl"
        name="imgfile"
        :headers="{ token: authStore.token || '' }"
        accept=".zip"
        directory-dnd
        :show-file-list="false"
        :before-upload="beforeUpload"
        @finish="handleUploadFinish"
      >
        <NButton type="warning" size="large" :loading="restoring">
          <template #icon>
            <SvgIcon icon="eos-icons:database-migration" />
          </template>
          {{ $t('apps.backupMigration.restore') }}
        </NButton>
      </NUpload>
    </div>
  </div>
</template>
