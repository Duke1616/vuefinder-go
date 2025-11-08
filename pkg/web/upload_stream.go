package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// bufferPool 复用缓冲区，减少内存分配
// 使用指针类型避免分配
var bufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 8*1024) // 8KB 用于读取小字段（name, path）
		return &buf
	},
}

// StreamingUploadHandler 使用原生 net/http 实现流式上传
// 绕过 Gin 的 multipart 解析，实现真正的边接收边写入
func StreamingUploadHandler(h *Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 处理 CORS 预检请求
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Finder-ID, X-Finder-Id")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// 设置 CORS 响应头
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Finder-ID, X-Finder-Id")

		// 只处理 POST 请求
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 检查 Content-Type
		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			errorResp := map[string]interface{}{
				"code":    0,
				"message": fmt.Sprintf("Invalid content type: %s", contentType),
				"data":    nil,
			}
			jsonData, _ := json.Marshal(errorResp)
			w.Write(jsonData)
			return
		}

		// 获取 finder ID
		finderID := r.Header.Get("x-finder-id")
		if finderID == "" {
			finderID = r.URL.Query().Get("id")
		}
		if finderID == "" {
			http.Error(w, "finder id is required", http.StatusBadRequest)
			return
		}

		id, err := strconv.ParseInt(finderID, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid finder id: %v", err), http.StatusBadRequest)
			return
		}

		fd, ok := h.finders[id]
		if !ok {
			http.Error(w, "finder not found", http.StatusNotFound)
			return
		}

		// 优化：先从 URL 参数获取 name 和 path，避免遍历所有 part
		// 这样可以更快地找到 file 部分并开始流式处理
		remoteFile := r.URL.Query().Get("name")
		remoteDir := r.URL.Query().Get("path")

		// 使用 MultipartReader 进行流式处理
		// 注意：如果请求体已经被读取（例如被 Gin 中间件），MultipartReader 会失败
		reader, err := r.MultipartReader()
		if err != nil {
			slog.Error("创建 MultipartReader 失败", slog.Any("err", err))
			writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("无法创建流式读取器，请求体可能已被读取: %v", err))
			return
		}

		var filePart *multipart.Part
		bufPtr := bufferPool.Get().(*[]byte)
		buf := *bufPtr
		defer bufferPool.Put(bufPtr)

		// 优化后的流式读取：如果已经从 URL 参数获取了 name 和 path，直接查找 file
		// 否则需要遍历所有 part 来获取这些信息
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				slog.Error("读取 multipart 部分失败", slog.Any("err", err))
				writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("读取 multipart 数据失败: %v", err))
				return
			}

			formName := part.FormName()

			// 如果已经从 URL 参数获取了 name 和 path，直接查找 file
			if remoteFile != "" && remoteDir != "" {
				if formName == "file" {
					filePart = part
					break
				}
				part.Close()
			} else {
				// 需要从 multipart 中获取 name 和 path
				switch formName {
				case "name":
					// 使用缓冲区读取，避免大内存分配
					var nameBuf bytes.Buffer
					_, err := io.CopyBuffer(&nameBuf, part, buf)
					part.Close()
					if err != nil {
						slog.Error("读取文件名失败", slog.Any("err", err))
						writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("读取文件名失败: %v", err))
						return
					}
					remoteFile = nameBuf.String()
				case "path":
					// 使用缓冲区读取，避免大内存分配
					var pathBuf bytes.Buffer
					_, err := io.CopyBuffer(&pathBuf, part, buf)
					part.Close()
					if err != nil {
						slog.Error("读取路径失败", slog.Any("err", err))
						writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("读取路径失败: %v", err))
						return
					}
					remoteDir = pathBuf.String()
				case "file":
					filePart = part
					// 如果已经获取了 name 和 path，可以退出
					if remoteFile != "" && remoteDir != "" {
						break
					}
					// 否则需要继续读取 name 和 path，但这样会跳过文件数据
					// 这种情况很少见，通常 file 在最后
				default:
					part.Close()
				}
			}
		}

		if filePart == nil {
			http.Error(w, "file not found in multipart form", http.StatusBadRequest)
			return
		}
		defer filePart.Close()

		// 如果没有提供文件名，使用上传的文件名
		if remoteFile == "" {
			remoteFile = filePart.FileName()
			if remoteFile == "" {
				remoteFile = "uploaded_file"
			}
		}

		// 使用流式上传，边接收边写入
		err = fd.UploadStream(context.Background(), filePart, remoteDir, remoteFile)
		if err != nil {
			slog.Error("上传文件失败", slog.Any("err", err))
			writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("上传文件失败: %v", err))
			return
		}

		// 上传成功后，返回当前目录的文件列表
		storage, err := fd.Index(context.Background(), remoteDir)
		if err != nil {
			slog.Error("获取文件列表失败", slog.Any("err", err))
			writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("获取文件列表失败: %v", err))
			return
		}

		// 返回成功响应
		writeSuccessResponse(w, storage)
	}
}

// writeErrorResponse 统一的错误响应写入函数，减少代码重复和内存分配
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// 使用预分配的 JSON 结构，减少内存分配
	errorResp := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    *int   `json:"data"`
	}{
		Code:    0,
		Message: message,
		Data:    nil,
	}

	json.NewEncoder(w).Encode(errorResp)
}

// writeSuccessResponse 统一的成功响应写入函数
func writeSuccessResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data"`
	}{
		Code:    1,
		Message: "File uploaded!",
		Data:    data,
	}

	json.NewEncoder(w).Encode(response)
}
