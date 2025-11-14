// WebSocket 文件上传 - 重写版本
export class WebSocketUploader {
  constructor(baseURL, finderId) {
    this.baseURL = baseURL;
    this.finderId = finderId;
    // 提升到 256KB，减少主线程事件与 JSON 开销（后端每 ~64KB 报告一次写入进度）
    this.chunkSize = 256 * 1024; // 256KB
    this._progressIntervalMs = 1000; // 进度节流间隔（1s 同步）
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
      // 重置内部状态，确保多次上传时能够正确启动
      this._startedSending = false;
      let _firstServerProgress = true;

      // 发送阶段的进度节流器
      let _lastEmit = 0;
      const progressState = {
        total: Number(file.size) || 0,
        written: 0,
      };
      const emitProgress = (extras) => {
        const now = Date.now();
        if (now - _lastEmit < this._progressIntervalMs) return;
        _lastEmit = now;
        if (onProgress) {
          const total = progressState.total || 0;
          const written = Math.min(progressState.written || 0, total);
          const payload = {
            bytesTotal: total,
            sftpWritten: written,
            sftpPercent: total > 0 ? ((written / total) * 100).toFixed(2) : '0.00',
          };
          onProgress(extras ? { ...payload, ...extras } : payload);
        }
      };

      ws.onopen = () => {
        // 发送开始消息，请求服务器返回可续传的偏移
        ws.send(JSON.stringify({
          type: 'start',
          id: uploadId,
          fileName: file.name,
          path: path,
          size: file.size,
        }));
      };

      ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data);
          
          switch (msg.type) {
            case 'progress': {
              // 统一写入进度状态并触发节流后的进度事件
              progressState.total = Number(msg.size) || Number(file.size) || 0;
              const sftpWritten = Number(msg.sftpWritten) || 0;
              const offset = Number(msg.offset) || 0; // 服务端已接收
              progressState.written = Math.max(progressState.written, sftpWritten);
              const extras = _firstServerProgress ? { resumeFrom: sftpWritten } : undefined;
              emitProgress(extras);
              _firstServerProgress = false;

              // 如果还未开始读取，收到初始 offset 后启动从该偏移读取
              if (!this._startedSending) {
                this._startedSending = true;
                this.readAndSendFile(ws, file, uploadId, offset, undefined);
              }
              break;
            }

            case 'success': {
              ws.close();
              if (onSuccess) {
                onSuccess(msg.data);
              }
              resolve(msg.data);
              break;
            }

            case 'error': {
              console.error(`[WebSocket上传] 上传失败: ${msg.error || '未知错误'}`);
              ws.close();
              const error = new Error(msg.error || '上传失败');
              if (onError) onError(error);
              reject(error);
              break;
            }

            default: {
              console.warn('未知的消息类型:', msg.type, msg);
              break;
            }
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
        // 1000: 正常关闭
        // 1005: 没有状态码（通常表示正常关闭，浏览器没有提供状态码，常见于上传完成后）
        // 只记录异常关闭的情况
        if (event.code !== 1000 && event.code !== 1005) {
          console.warn(`[WebSocket上传] 连接异常关闭: code=${event.code}, reason=${event.reason || '无原因'}`);
        }
        // 连接关闭后重置状态，避免影响下一次上传
        this._startedSending = false;
      };
    });
  }

  readAndSendFile(ws, file, uploadId, startOffset, onProgress) {
    const reader = new FileReader();
    let offset = Number(startOffset) || 0;

    const readNextChunk = () => {
      if (offset >= file.size) {
        // 文件读取完成，发送结束消息
        try {
          // 最后再发一次发送阶段进度，确保 UI 达到 100%
          if (onProgress) {
            onProgress(file.size);
          }
        } catch (_) {}
        ws.send(JSON.stringify({
          type: 'end',
          id: uploadId,
        }));
        return;
      }

      const chunk = file.slice(offset, offset + this.chunkSize);
      
      reader.onload = (e) => {
        const arrayBuffer = e.target.result;
        const bytes = new Uint8Array(arrayBuffer);
        
        // 转换为 base64
        let binary = '';
        for (let i = 0; i < bytes.length; i++) {
          binary += String.fromCharCode(bytes[i]);
        }
        const base64 = btoa(binary);

        // 发送数据块，携带当前偏移供服务端随机写入
        ws.send(JSON.stringify({
          type: 'chunk',
          id: uploadId,
          offset: offset,
          data: base64,
        }));

        offset += arrayBuffer.byteLength;
        
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
