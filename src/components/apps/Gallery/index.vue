<script setup lang="ts">
import { NAlert, NButton, NImage, NSpin, useDialog, useMessage } from 'naive-ui'
import { onMounted, ref } from 'vue'
import { getUserImagesList, updateGalleryFileType } from '@/api/panel/gallery'
import { deletes as deleteFiles } from '@/api/system/file'
import { RoundCardModal, SvgIcon } from '@/components/common'
import { t } from '@/locales'
import { copyToClipboard, timeFormat } from '@/utils/cmn'

const imageList = ref<File.Info[]>([])
const loading = ref(false)
const ms = useMessage()
const dialog = useDialog()
const infoModalState = ref<{ show: boolean, title: string, fileInfo: File.Info | null }>({
  show: false,
  title: '',
  fileInfo: null,
})

function imageUrl(src: string) {
  return `/uploads${src}`
}

async function getFileList() {
  loading.value = true
  await getUserImagesList<Common.ListResponse<File.Info[]>>({ page: 1, pageSize: 100 }).finally(() => {
    loading.value = false
  }).then((res) => {
    imageList.value = res.data.list
  }).catch((e) => {
    ms.error(e?.msg || String(e))
  })
}

async function copyImageUrl(text: string) {
  const res = await copyToClipboard(text)
  if (res)
    ms.success(t('apps.gallery.copyLink'))
  else
    ms.error(t('apps.gallery.copyLink'))
}

function handleSetWallpaper(item: File.Info) {
  if (item.id === undefined)
    return
  updateGalleryFileType({ id: item.id, type: 1 }).then(() => {
    ms.success(t('common.success'))
    getFileList()
  }).catch(e => ms.error(e?.msg || String(e)))
}

function handleDelete(item: File.Info) {
  if (item.id === undefined)
    return
  dialog.warning({
    title: t('common.warning'),
    content: t('common.deleteConfirm'),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: () => {
      deleteFiles([item.id as number]).then(() => {
        ms.success(t('common.deleteSuccess'))
        getFileList()
      }).catch(e => ms.error(e?.msg || String(e)))
    },
  })
}

function showInfo(item: File.Info) {
  infoModalState.value = { show: true, title: t('apps.uploadsFileManager.infoTitle'), fileInfo: item }
}

onMounted(() => {
  getFileList()
})
</script>

<template>
  <div class="p-2 h-full">
    <NAlert type="info" :bordered="false">
      {{ $t('apps.uploadsFileManager.alertText') }}
    </NAlert>

    <NSpin :show="loading">
      <div v-if="imageList.length === 0 && !loading" class="flex justify-center items-center h-[60vh]">
        {{ $t('apps.gallery.empty') }}
      </div>
      <div v-else class="grid gap-2 p-2" style="grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));">
        <div v-for="item in imageList" :key="item.id" class="flex flex-col justify-end mb-2 gap-2">
          <NImage
            :src="imageUrl(item.src)"
            lazy
            class="rounded-lg"
            object-fit="cover"
            style="width: 100%; height: 100px;"
            @click="showInfo(item)"
          />
          <div class="text-xs truncate" :title="item.fileName">
            {{ item.fileName }}
          </div>
          <div class="flex gap-1 flex-wrap">
            <NButton size="tiny" tertiary @click="handleSetWallpaper(item)">
              <template #icon>
                <SvgIcon icon="fa6:solid-image" />
              </template>
              {{ $t('apps.gallery.setWallpaper') }}
            </NButton>
            <NButton size="tiny" tertiary @click="copyImageUrl(imageUrl(item.src))">
              <template #icon>
                <SvgIcon icon="fa6:solid-link" />
              </template>
            </NButton>
            <NButton size="tiny" tertiary type="error" @click="handleDelete(item)">
              <template #icon>
                <SvgIcon icon="fa6:solid-trash-can" />
              </template>
            </NButton>
          </div>
        </div>
      </div>
    </NSpin>

    <RoundCardModal v-model:show="infoModalState.show" style="max-width: 300px;" size="small" :title="infoModalState.title">
      <div class="p-2 flex flex-col gap-2">
        <div class="flex justify-between">
          <span>{{ $t('apps.uploadsFileManager.fileName') }}</span>
          <span class="truncate max-w-[180px]" :title="infoModalState.fileInfo?.fileName">{{ infoModalState.fileInfo?.fileName }}</span>
        </div>
        <div class="flex justify-between">
          <span>{{ $t('apps.uploadsFileManager.path') }}</span>
          <span class="truncate max-w-[180px]">{{ infoModalState.fileInfo?.src }}</span>
        </div>
        <div class="flex justify-between">
          <span>{{ $t('apps.uploadsFileManager.uploadTime') }}</span>
          <span>{{ timeFormat(infoModalState.fileInfo?.createTime ?? '') }}</span>
        </div>
      </div>
    </RoundCardModal>
  </div>
</template>
