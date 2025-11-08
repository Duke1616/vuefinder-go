// WebSocket 文件上传 - 重写版本
export class WebSocketUploader {
  constructor(baseURL, finderId) {
    this.baseURL = baseURL;
    this.finderId = finderId;
    this.chunkSize = 64 * 1024; // 64KB
  }

  async uploadFile(file, path, onProgress, onSuccess, onError) {
    // 确保 file 是 File 对象
    if (!(file instanceof File)) {
      const error = new Error('File must be a File object');
      if (onError) onError(error);
      return Promise.reject(error);
    }

    return new Promise((resolve, reject) => {
      const wsURL = this.baseURL.replace(/^http/, 'ws') + '/upload/ws?id=' + this.finderId;
      const ws = new WebSocket(wsURL);
      const uploadId = `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
      let uploadedBytes = 0;

      ws.onopen = () => {
        console.log(`[WebSocket上传] 连接已建立，开始上传文件: ${file.name} (${(file.size / 1024 / 1024).toFixed(2)} MB)`);
        
        // 发送开始消息
        ws.send(JSON.stringify({
          type: 'start',
          id: uploadId,
          fileName: file.name,
          path: path,
          size: file.size,
        }));

        console.log(`[WebSocket上传] 已发送开始消息: id=${uploadId}, path=${path}`);

        // 开始读取文件并发送
        this.readAndSendFile(ws, file, uploadId, (bytes) => {
          uploadedBytes = bytes;
          const percent = ((bytes / file.size) * 100).toFixed(2);
          if (onProgress) {
            onProgress({
              bytesUploaded: bytes,
              bytesTotal: file.size,
            });
          }
        });
      };

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          
          if (msg.type === 'progress') {
            const percent = ((msg.offset / msg.size) * 100).toFixed(2);
            const uploadedMB = (msg.offset / 1024 / 1024).toFixed(2);
            const totalMB = (msg.size / 1024 / 1024).toFixed(2);
            
            // SFTP 写入进度
            const sftpWritten = msg.sftpWritten || 0;
            const sftpPercent = sftpWritten > 0 ? ((sftpWritten / msg.size) * 100).toFixed(2) : '0.00';
            const sftpMB = (sftpWritten / 1024 / 1024).toFixed(2);
            
            console.log(`[WebSocket上传] 接收进度: ${percent}% (${uploadedMB} MB / ${totalMB} MB)`);
            if (sftpWritten > 0) {
              console.log(`[WebSocket上传] SFTP写入进度: ${sftpPercent}% (${sftpMB} MB / ${totalMB} MB)`);
            }
            
            if (onProgress) {
              onProgress({
                bytesUploaded: msg.offset,
                bytesTotal: msg.size,
                sftpWritten: sftpWritten,
                sftpPercent: sftpPercent,
              });
            }
          } else if (msg.type === 'success') {
            // 显示 SFTP 写入时间信息
            if (msg.sftpWriteDuration) {
              const duration = (msg.sftpWriteDuration / 1000).toFixed(2);
              const speed = file.size / msg.sftpWriteDuration * 1000 / 1024 / 1024; // MB/s
              console.log(`[WebSocket上传] 上传成功: ${file.name}`);
              console.log(`[WebSocket上传] SFTP 写入耗时: ${duration} 秒 (${msg.sftpWriteDuration} 毫秒)`);
              console.log(`[WebSocket上传] SFTP 写入速度: ${speed.toFixed(2)} MB/s`);
              if (msg.sftpWriteStart && msg.sftpWriteEnd) {
                const startTime = new Date(msg.sftpWriteStart).toLocaleTimeString();
                const endTime = new Date(msg.sftpWriteEnd).toLocaleTimeString();
                console.log(`[WebSocket上传] SFTP 写入时间: ${startTime} - ${endTime}`);
              }
            } else {
              console.log(`[WebSocket上传] 上传成功: ${file.name}`);
            }
            ws.close();
            if (onSuccess) {
              onSuccess(msg.data);
            }
            resolve(msg.data);
          } else if (msg.type === 'error') {
            console.error(`[WebSocket上传] 上传失败: ${msg.error || '未知错误'}`);
            ws.close();
            const error = new Error(msg.error || '上传失败');
            if (onError) onError(error);
            reject(error);
          }
        } catch (err) {
          console.error(`[WebSocket上传] 解析服务器消息失败:`, err);
          ws.close();
          const error = new Error('解析服务器消息失败: ' + err.message);
          if (onError) onError(error);
          reject(error);
        }
      };

      ws.onerror = (error) => {
        console.error(`[WebSocket上传] WebSocket 错误:`, error);
        ws.close();
        if (onError) onError(error);
        reject(error);
      };

      ws.onclose = (event) => {
        if (event.code === 1000) {
          console.log(`[WebSocket上传] 连接正常关闭`);
        } else {
          console.warn(`[WebSocket上传] 连接关闭: code=${event.code}, reason=${event.reason || '无原因'}`);
        }
      };
    });
  }

  readAndSendFile(ws, file, uploadId, onProgress) {
    const reader = new FileReader();
    let offset = 0;

    let chunkCount = 0;
    const readNextChunk = () => {
      if (offset >= file.size) {
        // 文件读取完成，发送结束消息
        console.log(`[WebSocket上传] 文件读取完成，共发送 ${chunkCount} 个数据块，总大小: ${(offset / 1024 / 1024).toFixed(2)} MB`);
        ws.send(JSON.stringify({
          type: 'end',
          id: uploadId,
        }));
        return;
      }

      const chunk = file.slice(offset, offset + this.chunkSize);
      chunkCount++;
      
      reader.onload = (e) => {
        const arrayBuffer = e.target.result;
        const bytes = new Uint8Array(arrayBuffer);
        
        // 转换为 base64
        let binary = '';
        for (let i = 0; i < bytes.length; i++) {
          binary += String.fromCharCode(bytes[i]);
        }
        const base64 = btoa(binary);

        // 发送数据块
        ws.send(JSON.stringify({
          type: 'chunk',
          id: uploadId,
          data: base64,
        }));

        offset += arrayBuffer.byteLength;
        
        // 每 10 个块或每 1MB 记录一次日志
        if (chunkCount % 10 === 0 || offset % (1024 * 1024) < this.chunkSize) {
          const percent = ((offset / file.size) * 100).toFixed(2);
          console.log(`[WebSocket上传] 已发送 ${chunkCount} 个数据块，进度: ${percent}% (${(offset / 1024 / 1024).toFixed(2)} MB)`);
        }
        
        // 更新进度
        if (onProgress) {
          onProgress(offset);
        }

        // 继续读取下一块
        setTimeout(readNextChunk, 0);
      };

      reader.onerror = (error) => {
        console.error(`[WebSocket上传] 读取文件块失败:`, error);
        ws.send(JSON.stringify({
          type: 'error',
          id: uploadId,
          error: '读取文件失败: ' + error.message,
        }));
      };

      reader.readAsArrayBuffer(chunk);
    };

    // 开始读取
    readNextChunk();
  }
}
