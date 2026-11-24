<script setup lang="ts">
import { NAlert, NButton, NImage, NSpin, NUpload, useDialog, useMessage } from 'naive-ui'
import { onMounted, ref } from 'vue'
import { deletePublicImages, getPublicImagesList, publicGalleryUploadUrl } from '@/api/panel/gallery'
import { RoundCardModal, SvgIcon } from '@/components/common'
import { t } from '@/locales'
import { useAuthStore } from '@/store'
import { copyToClipboard, timeFormat } from '@/utils/cmn'

const imageList = ref<File.Info[]>([])
const loading = ref(false)
const ms = useMessage()
const dialog = useDialog()
const authStore = useAuthStore()
const selected = ref<Map<number, File.Info>>(new Map())
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
  await getPublicImagesList<Common.ListResponse<File.Info[]>>({ page: 1, pageSize: 100 }).finally(() => {
    loading.value = false
  }).then((res) => {
    imageList.value = res.data.list
    selected.value.clear()
  }).catch((e) => {
    ms.error(e?.msg || String(e))
  })
}

function handleUploadFinish({ event }: { event?: ProgressEvent }) {
  const target = (event as any)?.target
  if (!target?.response)
    return
  try {
    const res = JSON.parse(target.response)
    if (res.code === 0)
      ms.success(t('apps.publicGallery.uploadSuccess'))
    else
      ms.error(res.msg || t('apps.publicGallery.uploadFailed'))
  }
  catch {
    ms.error(t('apps.publicGallery.uploadFailed'))
  }
  getFileList()
}

function isSelected(item: File.Info) {
  return item.id !== undefined && selected.value.has(item.id)
}

function toggleSelect(item: File.Info) {
  if (item.id === undefined)
    return
  if (selected.value.has(item.id))
    selected.value.delete(item.id)
  else
    selected.value.set(item.id, item)
}

function handleDeleteSelected() {
  const ids = Array.from(selected.value.keys())
  if (ids.length === 0)
    return
  dialog.warning({
    title: t('common.warning'),
    content: t('apps.publicGallery.deleteConfirm'),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: () => {
      deletePublicImages(ids).then(() => {
        ms.success(t('common.deleteSuccess'))
        getFileList()
      }).catch(e => ms.error(e?.msg || String(e)))
    },
  })
}

async function copyImageUrl(text: string) {
  const res = await copyToClipboard(text)
  if (res)
    ms.success(t('apps.gallery.copyLink'))
  else
    ms.error(t('apps.gallery.copyLink'))
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
      {{ $t('apps.publicGallery.appName') }}
    </NAlert>

    <div class="flex justify-center gap-2 m-3">
      <NUpload
        :action="publicGalleryUploadUrl"
        name="imgfile"
        :headers="{ token: authStore.token || '' }"
        :data="{ type: 3 }"
        accept=".png,.jpg,.jpeg,.gif,.webp,.svg,.ico,.avif,.heic"
        :show-file-list="false"
        @finish="handleUploadFinish"
      >
        <NButton type="info">
          <template #icon>
            <SvgIcon icon="fa6:solid-cloud-arrow-up" />
          </template>
          {{ $t('apps.publicGallery.upload') }}
        </NButton>
      </NUpload>
      <NButton type="error" ghost :disabled="selected.size === 0" @click="handleDeleteSelected">
        <template #icon>
          <SvgIcon icon="fa6:solid-trash-can" />
        </template>
        {{ $t('common.delete') }} ({{ selected.size }})
      </NButton>
    </div>

    <NSpin :show="loading">
      <div v-if="imageList.length === 0 && !loading" class="flex justify-center items-center h-[60vh]">
        {{ $t('apps.publicGallery.empty') }}
      </div>
      <div v-else class="grid gap-2 p-2" style="grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));">
        <div v-for="item in imageList" :key="item.id" class="flex flex-col justify-end mb-2 gap-2">
          <NImage
            :src="imageUrl(item.src)"
            lazy
            class="rounded-lg cursor-pointer"
            object-fit="cover"
            style="width: 100%; height: 100px;"
            :class="isSelected(item) ? 'ring-2 ring-blue-500' : ''"
            @click="showInfo(item)"
          />
          <div class="flex gap-1 flex-wrap">
            <NButton size="tiny" tertiary @click="toggleSelect(item)">
              <template #icon>
                <SvgIcon :icon="isSelected(item) ? 'fa6:solid-square-check' : 'fa6:solid-square'" />
              </template>
            </NButton>
            <NButton size="tiny" tertiary @click.stop="copyImageUrl(imageUrl(item.src))">
              <template #icon>
                <SvgIcon icon="fa6:solid-link" />
              </template>
            </NButton>
          </div>
          <div class="text-xs truncate" :title="item.fileName">
            {{ item.fileName }} · {{ timeFormat(item.createTime ?? '') }}
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
