<template>
  <div class="wrapper">
    <vue-finder
      ref="vuefinderRef"
      id="vuefinder"
      :driver="driver"
      :config="{
        theme: 'dark',
        maxFileSize: '500mb',
        fullScreen: true,
      }"
      :custom-uploader="customUploader"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from "vue";
import { RemoteDriver } from 'vuefinder';
import { WebSocketUploader } from './websocket-upload.js';

const vuefinderRef = ref(null);

// 配置常量
const BASE_URL = "http://127.0.0.1:8350/api/finder";
const FINDER_ID = 20;

// 扩展 RemoteDriver 以支持自定义下载（使用原生下载，支持 Range，避免内存聚合）
class CustomRemoteDriver extends RemoteDriver {
  async download(filePath) {
    try {
      const url = `${BASE_URL}/download?path=${encodeURIComponent(filePath)}&id=${encodeURIComponent(String(FINDER_ID))}`;
      const link = document.createElement('a');
      link.href = url;
      link.rel = 'noopener noreferrer';
      link.style.display = 'none';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
    } catch (error) {
      console.error('下载失败:', error);
      alert(`下载失败: ${error.message}`);
      throw error;
    }
  }
}

// 使用自定义的 RemoteDriver
const driver = new CustomRemoteDriver({
  baseURL: BASE_URL,
  headers: {
    'X-Finder-ID': FINDER_ID,
  },
  retry: 0,
  url: {
    list: '/files',
    upload: '/upload',
    delete: '/delete',
    rename: '/rename',
    copy: '/copy',
    move: '/move',
    archive: '/archive',
    unarchive: '/unarchive',
    createFile: '/new_file',
    createFolder: '/new_folder',
    preview: '/preview',
    download: '/download',
    search: '/search',
    save: '/save',
  },
});

// 创建 WebSocket 上传器实例
const wsUploader = new WebSocketUploader(BASE_URL, FINDER_ID);

// 使用 VueFinder 的 customUploader prop 配置 WebSocket 上传
// 参考: https://github.com/n1crack/vuefinder/blob/master/docs/api-reference/props.md
const customUploader = (uppy, context) => {  
  // 监听上传事件
  uppy.on('upload', async (fileIds) => {
    const targetPath = context.getTargetPath() || '';
    const allFiles = uppy.getFiles();
    
    // 获取要上传的文件列表（保留 fallback 逻辑）
    const fileList = typeof fileIds === 'string'
      ? (allFiles[fileIds] ? [allFiles[fileIds]] : Object.values(allFiles))
      : Array.isArray(fileIds)
        ? fileIds.map(id => allFiles[id]).filter(Boolean)
        : Object.values(allFiles);
    
    // 使用 WebSocket 上传每个文件
    for (const uppyFile of fileList) {
      if (!uppyFile) continue;
      
      const file = uppyFile.data;
      if (!file || !(file instanceof File)) {
        uppy.emit('upload-error', uppyFile, new Error('无效的文件对象'));
        continue;
      }
      
      try {
        await wsUploader.uploadFile(
          file,
          targetPath,
          (progress) => {
            uppy.emit('upload-progress', uppyFile, {
              bytesUploaded: progress.bytesUploaded,
              bytesTotal: progress.bytesTotal,
            });
          },
          (data) => {
            uppy.emit('upload-success', uppyFile, {
              status: 200,
              body: data,
            });
          },
          (error) => {
            uppy.emit('upload-error', uppyFile, error);
          }
        );
      } catch (error) {
        uppy.emit('upload-error', uppyFile, error);
      }
    }
  });
};

// 下载处理函数
const handleDownload = async (filePath) => {
  await driver.download(filePath);
};

// 下载链接点击处理函数
const handleDownloadClick = async (event) => {
  const target = event.target.closest('a');
  if (!target) return;
  
  const href = target.getAttribute('href');
  if (!href?.includes('/api/finder/download')) return;
  
  event.preventDefault();
  event.stopPropagation();
  
  const url = new URL(href, window.location.origin);
  const filePath = url.searchParams.get('path');
  if (!filePath) return;
  
  await handleDownload(filePath);
};

onMounted(() => {
  document.addEventListener('click', handleDownloadClick, true);
});

onUnmounted(() => {
  document.removeEventListener('click', handleDownloadClick, true);
});
</script>

<style lang="scss">
body {
  margin: 0;
  background: #eeeeee;
}
.wrapper {
  max-width: 800px;
  margin: 80px auto;
}
.btn {
  display: block;
  margin: 20px auto;
  padding: 10px 20px;
  border: 1px solid #ccc;
  border-radius: 5px;
  background: #fff;
  cursor: pointer;
  outline: none;
}
</style>
