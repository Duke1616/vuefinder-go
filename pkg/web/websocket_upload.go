package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Duke1616/vuefinder-go/pkg/finder"
	"github.com/gorilla/websocket"
)

const (
	// WebSocket 消息类型
	wsMsgTypeStart    = "start"    // 开始上传
	wsMsgTypeChunk    = "chunk"    // 文件数据块
	wsMsgTypeEnd      = "end"      // 上传结束
	wsMsgTypeProgress = "progress" // 进度更新（服务器 -> 客户端）
	wsMsgTypeSuccess  = "success"  // 上传成功
	wsMsgTypeError    = "error"    // 上传失败

	// WebSocket 配置常量
	readDeadline     = 60 * time.Second // 读取超时时间
	messageQueueSize = 100              // 消息队列大小
)

// UploadMessage WebSocket 消息结构
type UploadMessage struct {
	Type              string          `json:"type"`              // 消息类型
	ID                string          `json:"id"`                // 上传任务 ID
	FileName          string          `json:"fileName"`          // 文件名
	Path              string          `json:"path"`              // 目标路径
	Data              json.RawMessage `json:"data"`              // 数据（base64 编码的文件块或元数据）
	Size              int64           `json:"size"`              // 文件总大小
	Offset            int64           `json:"offset"`            // 当前偏移量（已接收的字节数）
	SFTPWritten       int64           `json:"sftpWritten"`       // 已写入 SFTP 的字节数（实时进度）
	Error             string          `json:"error"`             // 错误信息
	SFTPWriteStart    int64           `json:"sftpWriteStart"`    // SFTP 写入开始时间（Unix 毫秒时间戳）
	SFTPWriteEnd      int64           `json:"sftpWriteEnd"`      // SFTP 写入结束时间（Unix 毫秒时间戳）
	SFTPWriteDuration int64           `json:"sftpWriteDuration"` // SFTP 写入耗时（毫秒）
}

// UploadSession 上传会话
type UploadSession struct {
	ID       string
	FileName string
	Path     string
	Size     int64
	Writer   io.WriteCloser
	Finder   finder.Finder
	Offset   int64
	mu       sync.RWMutex // 保护 Offset 的并发访问
}

// uploadWriter 边接边写：通过 io.Pipe 将收到的数据实时写入 SFTP
type uploadWriter struct {
	finder         finder.Finder
	remoteDir      string
	remoteFile     string
	ctx            context.Context
	totalSize      int64

	pr  *io.PipeReader
	pw  *io.PipeWriter
	done chan error

	closed         bool
	sftpWriteStart time.Time                  // SFTP 写入开始时间
	sftpWriteEnd   time.Time                  // SFTP 写入结束时间
	sftpWritten    int64                      // 已写入 SFTP 的字节数
	onProgress     func(written, total int64) // SFTP 写入进度回调
	mu             sync.Mutex
}

func (w *uploadWriter) startWriterGoroutine() {
	w.sftpWriteStart = time.Now()
	go func() {
		// 进度回调：更新 sftpWritten 并透传
		progressCallback := func(written, total int64) {
			w.mu.Lock()
			w.sftpWritten = written
			w.mu.Unlock()
			if w.onProgress != nil {
				w.onProgress(written, total)
			}
		}

		err := w.finder.UploadStreamWithProgress(w.ctx, w.pr, w.remoteDir, w.remoteFile, w.totalSize, progressCallback)
		w.sftpWriteEnd = time.Now()

		// 确保最终写入字节与总大小一致（若后端未完整回调）
		w.mu.Lock()
		if w.sftpWritten < w.totalSize {
			w.sftpWritten = w.totalSize
		}
		w.mu.Unlock()

		w.done <- err
		close(w.done)
	}()
}

func (w *uploadWriter) Write(p []byte) (n int, err error) {
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	if w.pw == nil {
		return 0, fmt.Errorf("writer not initialized")
	}
	return w.pw.Write(p)
}

func (w *uploadWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	// 关闭写入端，通知后台写入完成
	if w.pw != nil {
		_ = w.pw.Close()
	}
	// 等待后台 goroutine 结束
	if w.done != nil {
		if err, ok := <-w.done; ok {
			return err
		}
	}
	return nil
}

// GetSFTPWritten 获取已写入 SFTP 的字节数
func (w *uploadWriter) GetSFTPWritten() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.sftpWritten
}

// SetProgressCallback 设置进度回调函数
func (w *uploadWriter) SetProgressCallback(callback func(written, total int64)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onProgress = callback
}

// GetSFTPWriteDuration 获取 SFTP 写入耗时（毫秒）
func (w *uploadWriter) GetSFTPWriteDuration() int64 {
	if w.sftpWriteStart.IsZero() || w.sftpWriteEnd.IsZero() {
		return 0
	}
	return w.sftpWriteEnd.Sub(w.sftpWriteStart).Milliseconds()
}

// GetSFTPWriteStart 获取 SFTP 写入开始时间（Unix 毫秒时间戳）
func (w *uploadWriter) GetSFTPWriteStart() int64 {
	if w.sftpWriteStart.IsZero() {
		return 0
	}
	return w.sftpWriteStart.UnixMilli()
}

// GetSFTPWriteEnd 获取 SFTP 写入结束时间（Unix 毫秒时间戳）
func (w *uploadWriter) GetSFTPWriteEnd() int64 {
	if w.sftpWriteEnd.IsZero() {
		return 0
	}
	return w.sftpWriteEnd.UnixMilli()
}

// createUploadWriter 创建上传写入器
func createUploadWriter(ctx context.Context, fd finder.Finder, remoteDir, remoteFile string, totalSize int64) (io.WriteCloser, error) {
	pr, pw := io.Pipe()
	uw := &uploadWriter{
		finder:     fd,
		remoteDir:  remoteDir,
		remoteFile: remoteFile,
		ctx:        ctx,
		totalSize:  totalSize,
		pr:         pr,
		pw:         pw,
		done:       make(chan error, 1),
	}
	uw.startWriterGoroutine()
	return uw, nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源，生产环境应该限制
		return true
	},
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// getFinderID 从请求中获取 finder ID
func getFinderID(r *http.Request) string {
	if id := r.URL.Query().Get("id"); id != "" {
		return id
	}
	return r.Header.Get("x-finder-id")
}

// UploadHandler 处理 WebSocket 文件上传
func UploadHandler(h *Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 升级为 WebSocket 连接
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("WebSocket 升级失败", slog.Any("err", err))
			return
		}
		defer conn.Close()

		// 获取 finder ID
		finderID := getFinderID(r)
		if finderID == "" {
			sendErrorSync(conn, "", "finder id is required")
			return
		}

		id, err := strconv.ParseInt(finderID, 10, 64)
		if err != nil {
			sendErrorSync(conn, "", fmt.Sprintf("invalid finder id: %v", err))
			return
		}

		fd, ok := h.finders[id]
		if !ok {
			sendErrorSync(conn, "", "finder not found")
			return
		}

		// 创建消息发送队列，用于序列化所有 WebSocket 写入
		msgChan := make(chan UploadMessage, messageQueueSize)
		done := make(chan struct{})

		// 启动消息发送 goroutine，序列化所有 WebSocket 写入
		go func() {
			defer close(done)
			for msg := range msgChan {
				if err = conn.WriteJSON(msg); err != nil {
					slog.Error("发送 WebSocket 消息失败", slog.Any("err", err))
					return
				}
			}
		}()

		// 存储活跃的上传会话（使用带锁的 map）
		sessions := make(map[string]*UploadSession)
		sessionsMu := sync.RWMutex{}

		// 设置读取超时和关闭处理
		if err = conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
			slog.Warn("设置读取超时失败", slog.Any("err", err))
		}
		conn.SetPongHandler(func(string) error {
			if err = conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
				slog.Warn("重置读取超时失败", slog.Any("err", err))
			}
			return nil
		})

		// 使用请求上下文，支持取消操作
		ctx := r.Context()

		// 读取消息循环
		for {
			var msg UploadMessage
			err = conn.ReadJSON(&msg)
			if err != nil {
				// 检查是否是正常的关闭错误
				if isNormalClose(err) {
					slog.Debug("WebSocket 正常关闭", slog.Any("err", err))
				} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					slog.Error("WebSocket 读取错误", slog.Any("err", err))
				} else {
					slog.Debug("WebSocket 连接关闭", slog.Any("err", err))
				}
				break
			}

			// 重置读取超时
			if err = conn.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
				slog.Warn("重置读取超时失败", slog.Any("err", err))
			}

			switch msg.Type {
			case wsMsgTypeStart:
				// 开始上传
				if err = handleStartUpload(ctx, msgChan, &msg, fd, sessions, &sessionsMu); err != nil {
					slog.Error("处理开始上传失败", slog.Any("err", err))
					sendError(msgChan, msg.ID, err.Error())
				}
			case wsMsgTypeChunk:
				// 文件数据块
				if err = handleChunk(msgChan, &msg, sessions, &sessionsMu); err != nil {
					slog.Error("处理数据块失败", slog.Any("err", err))
					sendError(msgChan, msg.ID, err.Error())
					// 清理会话
					cleanupSession(sessions, &sessionsMu, msg.ID)
				}
			case wsMsgTypeEnd:
				// 上传结束
				if err = handleEndUpload(ctx, msgChan, &msg, sessions, &sessionsMu); err != nil {
					slog.Error("处理上传结束失败", slog.Any("err", err))
					sendError(msgChan, msg.ID, err.Error())
				}
				// 清理会话
				cleanupSession(sessions, &sessionsMu, msg.ID)
			default:
				sendError(msgChan, msg.ID, fmt.Sprintf("unknown message type: %s", msg.Type))
			}
		}

		// 清理所有会话
		sessionsMu.Lock()
		for _, session := range sessions {
			session.Writer.Close()
		}
		sessionsMu.Unlock()

		// 关闭消息队列，等待发送 goroutine 完成
		close(msgChan)
		<-done
	}
}

// cleanupSession 清理会话
func cleanupSession(sessions map[string]*UploadSession, mu *sync.RWMutex, id string) {
	mu.Lock()
	defer mu.Unlock()
	if session, ok := sessions[id]; ok {
		session.Writer.Close()
		delete(sessions, id)
	}
}

// handleStartUpload 处理开始上传
func handleStartUpload(ctx context.Context, msgChan chan<- UploadMessage, msg *UploadMessage, fd finder.Finder, sessions map[string]*UploadSession, mu *sync.RWMutex) error {
	// 创建流式写入器（传入总大小用于进度计算）
	writer, err := createUploadWriter(ctx, fd, msg.Path, msg.FileName, msg.Size)
	if err != nil {
		return fmt.Errorf("创建上传流失败: %w", err)
	}

	session := &UploadSession{
		ID:       msg.ID,
		FileName: msg.FileName,
		Path:     msg.Path,
		Size:     msg.Size,
		Writer:   writer,
		Finder:   fd,
		Offset:   0,
	}

	mu.Lock()
	sessions[msg.ID] = session
	mu.Unlock()

	// 设置进度回调，通过 WebSocket 发送进度更新
	// 注意：需要在 Close() 之前设置，因为 Close() 会触发 SFTP 写入
	if uploader, ok := writer.(*uploadWriter); ok {
		uploader.SetProgressCallback(func(written, total int64) {
			// 读取当前接收进度（需要加锁）
			session.mu.RLock()
			offset := session.Offset
			session.mu.RUnlock()

			// 通过消息队列发送 SFTP 写入进度更新（非阻塞）
			select {
			case msgChan <- UploadMessage{
				Type:        wsMsgTypeProgress,
				ID:          msg.ID,
				Size:        total,
				Offset:      offset,  // 接收进度
				SFTPWritten: written, // SFTP 写入进度
			}:
			default:
				// 如果队列满了，记录警告但不阻塞
				slog.Warn("消息队列已满，跳过进度更新", slog.String("id", msg.ID))
			}
		})
	}

	// 发送确认消息
	msgChan <- UploadMessage{
		Type:   wsMsgTypeProgress,
		ID:     msg.ID,
		Size:   msg.Size,
		Offset: 0,
	}
	return nil
}

// handleChunk 处理文件数据块
func handleChunk(msgChan chan<- UploadMessage, msg *UploadMessage, sessions map[string]*UploadSession, mu *sync.RWMutex) error {
	mu.RLock()
	session, ok := sessions[msg.ID]
	mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", msg.ID)
	}

	// 解码 base64 数据
	var base64Str string
	if err := json.Unmarshal(msg.Data, &base64Str); err != nil {
		return fmt.Errorf("解析 base64 数据失败: %w", err)
	}

	chunkData, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return fmt.Errorf("解码 base64 失败: %w", err)
	}

	// 写入数据
	n, err := session.Writer.Write(chunkData)
	if err != nil {
		return fmt.Errorf("写入数据失败: %w", err)
	}

	// 更新接收进度（需要加锁）
	session.mu.Lock()
	session.Offset += int64(n)
	offset := session.Offset
	session.mu.Unlock()

	// 获取已写入 SFTP 的字节数（实时进度）
	var sftpWritten int64
	if writer, ok := session.Writer.(*uploadWriter); ok {
		sftpWritten = writer.GetSFTPWritten()
	}

	// 发送进度更新，包含 SFTP 写入进度
	msgChan <- UploadMessage{
		Type:        wsMsgTypeProgress,
		ID:          msg.ID,
		Size:        session.Size,
		Offset:      offset,
		SFTPWritten: sftpWritten,
	}
	return nil
}

// handleEndUpload 处理上传结束
func handleEndUpload(ctx context.Context, msgChan chan<- UploadMessage, msg *UploadMessage, sessions map[string]*UploadSession, mu *sync.RWMutex) error {
	mu.RLock()
	session, ok := sessions[msg.ID]
	mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", msg.ID)
	}

	session.mu.RLock()
	totalSize := session.Offset
	session.mu.RUnlock()

	// 在关闭之前保存 uploadWriter 引用，以便获取 SFTP 写入时间信息
	var sftpWriteStart, sftpWriteEnd, sftpWriteDuration int64
	writer, isUploadWriter := session.Writer.(*uploadWriter)

	// 关闭写入器（这会触发 SFTP 写入）
	if err := session.Writer.Close(); err != nil {
		return fmt.Errorf("关闭写入器失败: %w", err)
	}

	// 获取 SFTP 写入时间信息
	if isUploadWriter && writer != nil {
		sftpWriteStart = writer.GetSFTPWriteStart()
		sftpWriteEnd = writer.GetSFTPWriteEnd()
		sftpWriteDuration = writer.GetSFTPWriteDuration()
	}

	slog.Info("文件上传成功",
		slog.String("id", msg.ID),
		slog.String("fileName", session.FileName),
		slog.String("path", session.Path),
		slog.Int64("size", totalSize),
		slog.Int64("sftpWriteDuration", sftpWriteDuration),
	)

	// 获取文件列表（使用请求上下文）
	storage, err := session.Finder.Index(ctx, session.Path)
	if err != nil {
		return fmt.Errorf("获取文件列表失败: %w", err)
	}

	// 发送成功消息，包含 SFTP 写入时间信息
	successData, err := json.Marshal(storage)
	if err != nil {
		return fmt.Errorf("序列化文件列表失败: %w", err)
	}
	msgChan <- UploadMessage{
		Type:              wsMsgTypeSuccess,
		ID:                msg.ID,
		Data:              successData,
		SFTPWriteStart:    sftpWriteStart,
		SFTPWriteEnd:      sftpWriteEnd,
		SFTPWriteDuration: sftpWriteDuration,
	}
	return nil
}

// sendError 通过消息队列发送错误消息
func sendError(msgChan chan<- UploadMessage, id, errorMsg string) {
	select {
	case msgChan <- UploadMessage{
		Type:  wsMsgTypeError,
		ID:    id,
		Error: errorMsg,
	}:
	default:
		// 如果队列满了，记录警告
		slog.Warn("消息队列已满，无法发送错误消息", slog.String("id", id), slog.String("error", errorMsg))
	}
}

// sendErrorSync 同步发送错误消息（用于初始化阶段，消息队列还未创建时）
func sendErrorSync(conn *websocket.Conn, id, errorMsg string) {
	if err := conn.WriteJSON(UploadMessage{
		Type:  wsMsgTypeError,
		ID:    id,
		Error: errorMsg,
	}); err != nil {
		slog.Error("同步发送错误消息失败", slog.Any("err", err), slog.String("id", id))
	}
}

// isNormalClose 检查是否是正常的关闭错误
// 1005 (no status) 通常表示正常关闭，不应该记录为错误
func isNormalClose(err error) bool {
	if err == nil {
		return false
	}
	// 检查是否是正常的关闭错误（1000 或 1005）
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, 1005)
}
