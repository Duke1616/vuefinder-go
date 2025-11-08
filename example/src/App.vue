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
import { ref, onMounted, onUnmounted, nextTick } from "vue";
import { RemoteDriver } from 'vuefinder';

const vuefinderRef = ref(null);

import { WebSocketUploader } from './websocket-upload.js';

// 使用标准的 RemoteDriver
const driver = new RemoteDriver({
  baseURL: "http://127.0.0.1:8350/api/finder",
  headers: {
    'X-Finder-ID': 20,
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
// 直接从配置中获取 baseURL 和 finderId
const baseURL = "http://127.0.0.1:8350/api/finder";
const finderId = 20;
const wsUploader = new WebSocketUploader(baseURL, finderId);

// 使用 VueFinder 的 customUploader prop 配置 WebSocket 上传
// 参考: https://github.com/n1crack/vuefinder/blob/master/docs/api-reference/props.md
const customUploader = (uppy, context) => {
  console.log('customUploader called', { uppy, context });
  
  // 等待插件注册完成
  setTimeout(() => {
    // 获取 XHRUpload 插件
    const xhrPlugin = uppy.getPlugin('XHRUpload');
    if (xhrPlugin) {
      console.log('XHRUpload plugin found, disabling it');
      xhrPlugin.setOptions({ disabled: true });
    } else {
      console.warn('XHRUpload plugin not found, will use event listener');
    }
  }, 100);
  
  // 监听上传事件
  uppy.on('upload', async (fileIds) => {
    console.log('upload event triggered', fileIds);
    
    // 使用 context.getTargetPath() 获取目标路径
    const targetPath = context.getTargetPath() || '';
    console.log('Target path:', targetPath);
    
    // 获取所有待上传的文件
    const allFiles = uppy.getFiles();
    console.log('All files:', allFiles);
    console.log('All files keys:', Object.keys(allFiles));
    
    // fileIds 可能是字符串（单个文件 ID）或数组（多个文件 ID）
    let fileList = [];
    
    if (typeof fileIds === 'string') {
      // 如果是字符串，是文件 ID，直接从 allFiles 对象中获取
      const file = allFiles[fileIds];
      if (file) {
        fileList = [file];
      } else {
        console.warn('File not found in allFiles:', fileIds, 'Available:', Object.keys(allFiles));
        // 如果找不到，尝试所有文件
        fileList = Object.values(allFiles);
      }
    } else if (Array.isArray(fileIds)) {
      // 如果是数组，遍历获取文件
      fileList = fileIds.map(id => allFiles[id]).filter(Boolean);
    } else {
      // 否则，获取所有文件
      fileList = Object.values(allFiles);
    }
    
    console.log('File list to process:', fileList);
    
    // 使用 WebSocket 上传每个文件
    for (const uppyFile of fileList) {
      if (!uppyFile) {
        console.warn('Invalid file object:', uppyFile);
        continue;
      }
      
      console.log('Processing file:', uppyFile);
      
      try {
        // uppy 文件对象的 data 属性是 File 对象
        const file = uppyFile.data;
        console.log('File data:', file, 'is File:', file instanceof File);
        
        if (!(file instanceof File)) {
          throw new Error(`文件 ${uppyFile.name} 不是有效的 File 对象`);
        }
        
        console.log(`[App.vue] 开始上传文件: ${uppyFile.name} (${(file.size / 1024 / 1024).toFixed(2)} MB) 到路径: ${targetPath}`);
        
        await wsUploader.uploadFile(
          file,
          targetPath,
          // 进度回调
          (progress) => {
            const percent = ((progress.bytesUploaded / progress.bytesTotal) * 100).toFixed(2);
            // 进度日志在 websocket-upload.js 中已记录，这里只更新 uppy 状态
            uppy.emit('upload-progress', uppyFile, {
              bytesUploaded: progress.bytesUploaded,
              bytesTotal: progress.bytesTotal,
            });
          },
          // 成功回调
          (data) => {
            console.log(`[App.vue] 文件上传成功: ${uppyFile.name}`);
            uppy.emit('upload-success', uppyFile, {
              status: 200,
              body: data,
            });
          },
          // 错误回调
          (error) => {
            console.error(`[App.vue] 文件上传失败: ${uppyFile.name}`, error);
            uppy.emit('upload-error', uppyFile, error);
          }
        );
      } catch (error) {
        console.error('上传文件失败:', error);
        uppy.emit('upload-error', uppyFile, error);
      }
    }
  });
  
  console.log('customUploader setup complete');
};

// 自定义下载处理 - 使用 fetch 传递 header
const downloadFile = async (filePath, fileName) => {
  try {
    const url = `http://127.0.0.1:8350/api/finder/download?path=${encodeURIComponent(filePath)}`;
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        'X-Finder-ID': '20',
      },
    });
    
    if (!response.ok) {
      const error = await response.json();
      alert(`下载失败: ${error.message || '未知错误'}`);
      return;
    }
    
    // 获取文件名
    const contentDisposition = response.headers.get('Content-Disposition');
    let finalFileName = fileName || filePath.split('/').pop();
    if (contentDisposition) {
      const fileNameMatch = contentDisposition.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/);
      if (fileNameMatch && fileNameMatch[1]) {
        finalFileName = fileNameMatch[1].replace(/['"]/g, '');
      }
    }
    
    // 下载文件
    const blob = await response.blob();
    const downloadUrl = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = downloadUrl;
    link.setAttribute('download', finalFileName);
    link.style.display = 'none';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(downloadUrl);
  } catch (error) {
    console.error('下载失败:', error);
    alert(`下载失败: ${error.message}`);
  }
};

onMounted(async () => {
  // 等待组件挂载完成
  await nextTick();
  
  // 获取 vuefinder 实例并监听事件
  if (vuefinderRef.value) {
    const vuefinderInstance = vuefinderRef.value;
    
    // 监听上传完成事件
    if (vuefinderInstance.emitter) {
      vuefinderInstance.emitter.on('vf-upload-complete', (files) => {
        console.log('上传完成:', files);
      });
      
      // 监听其他事件
      vuefinderInstance.emitter.on('vf-toast-push', (data) => {
        console.log('Toast:', data);
      });
    }
    
    // 如果 vuefinder 使用 uppy，也可以直接监听 uppy 的事件
    // 注意：这需要访问内部的 uppy 实例
  }
  
  // 监听所有链接点击事件，拦截下载链接
  const handleClick = async (event) => {
    const target = event.target.closest('a');
    if (!target) return;
    
    const href = target.getAttribute('href');
    if (!href || !href.includes('/api/finder/download')) return;
    
    // 阻止默认行为
    event.preventDefault();
    event.stopPropagation();
    
    // 从 href 中提取 path 参数
    const url = new URL(href, window.location.origin);
    const filePath = url.searchParams.get('path');
    if (!filePath) return;
    
    // 执行自定义下载
    await downloadFile(filePath);
  };
  
  // 监听文档点击事件
  document.addEventListener('click', handleClick, true);
  
  // 保存清理函数
  window._downloadCleanup = () => {
    document.removeEventListener('click', handleClick, true);
  };
});

onUnmounted(() => {
  // 清理事件监听
  if (window._downloadCleanup) {
    window._downloadCleanup();
    delete window._downloadCleanup;
  }
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
