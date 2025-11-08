package web

import (
	"bytes"
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
)

// WebSocketUploadMessage WebSocket 消息结构
type WebSocketUploadMessage struct {
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

// WebSocketUploadSession 上传会话
type WebSocketUploadSession struct {
	ID                string
	FileName          string
	Path              string
	Size              int64
	Writer            io.WriteCloser
	Finder            finder.Finder
	Offset            int64
	lastLoggedPercent int // 上次记录的百分比（用于控制日志频率）
}

// uploadWriter 实现 io.WriteCloser，用于流式写入到 SFTP
// 使用 bytes.Buffer 累积数据，在 Close 时一次性写入
type uploadWriter struct {
	finder         finder.Finder
	remoteDir      string
	remoteFile     string
	buffer         *bytes.Buffer
	ctx            context.Context
	closed         bool
	sftpWriteStart time.Time                  // SFTP 写入开始时间
	sftpWriteEnd   time.Time                  // SFTP 写入结束时间
	sftpWritten    int64                      // 已写入 SFTP 的字节数
	onProgress     func(written, total int64) // SFTP 写入进度回调
	mu             sync.Mutex
}

func (w *uploadWriter) Write(p []byte) (n int, err error) {
	if w.buffer == nil {
		w.buffer = &bytes.Buffer{}
	}
	return w.buffer.Write(p)
}

func (w *uploadWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	if w.buffer == nil || w.buffer.Len() == 0 {
		return nil
	}

	// 记录 SFTP 写入开始时间
	w.sftpWriteStart = time.Now()

	// 创建带进度跟踪的 reader
	reader := bytes.NewReader(w.buffer.Bytes())
	totalSize := int64(w.buffer.Len())

	// 创建进度回调函数
	progressCallback := func(written, total int64) {
		w.mu.Lock()
		w.sftpWritten = written
		w.mu.Unlock()

		// 调用外部进度回调（通过 WebSocket 发送）
		if w.onProgress != nil {
			w.onProgress(written, total)
		}

		// 记录日志以便调试
		percent := float64(written) / float64(total) * 100
		slog.Info("SFTP 写入进度",
			slog.Int64("written", written),
			slog.Int64("total", total),
			slog.Float64("percent", percent),
		)
	}

	// 直接调用 UploadStreamWithProgress（现在接口中已定义）
	err := w.finder.UploadStreamWithProgress(w.ctx, reader, w.remoteDir, w.remoteFile, totalSize, progressCallback)

	// 记录 SFTP 写入结束时间
	w.sftpWriteEnd = time.Now()

	// 更新最终写入字节数
	w.mu.Lock()
	w.sftpWritten = totalSize
	w.mu.Unlock()

	return err
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
func createUploadWriter(fd finder.Finder, remoteDir, remoteFile string) (io.WriteCloser, error) {
	return &uploadWriter{
		finder:     fd,
		remoteDir:  remoteDir,
		remoteFile: remoteFile,
		ctx:        context.Background(),
	}, nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有来源，生产环境应该限制
		return true
	},
}

// WebSocketUploadHandler 处理 WebSocket 文件上传
func WebSocketUploadHandler(h *Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 升级为 WebSocket 连接
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("WebSocket 升级失败", slog.Any("err", err))
			return
		}
		defer conn.Close()

		// 获取 finder ID
		finderID := r.URL.Query().Get("id")
		if finderID == "" {
			finderID = r.Header.Get("x-finder-id")
		}
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
		msgChan := make(chan WebSocketUploadMessage, 100)
		done := make(chan struct{})

		// 启动消息发送 goroutine，序列化所有 WebSocket 写入
		go func() {
			defer close(done)
			for msg := range msgChan {
				if err := conn.WriteJSON(msg); err != nil {
					slog.Error("发送 WebSocket 消息失败", slog.Any("err", err))
					return
				}
			}
		}()

		// 存储活跃的上传会话
		sessions := make(map[string]*WebSocketUploadSession)

		// 设置读取超时和关闭处理
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		// 读取消息循环
		for {
			var msg WebSocketUploadMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					slog.Error("WebSocket 读取错误", slog.Any("err", err))
				} else if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					slog.Info("WebSocket 正常关闭")
				} else {
					slog.Error("WebSocket 读取消息失败", slog.Any("err", err))
				}
				break
			}

			// 重置读取超时
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			switch msg.Type {
			case wsMsgTypeStart:
				// 开始上传
				err := handleStartUpload(msgChan, &msg, fd, sessions)
				if err != nil {
					slog.Error("处理开始上传失败", slog.Any("err", err))
					sendError(msgChan, msg.ID, err.Error())
				}
			case wsMsgTypeChunk:
				// 文件数据块
				err := handleChunk(msgChan, &msg, sessions)
				if err != nil {
					slog.Error("处理数据块失败", slog.Any("err", err))
					sendError(msgChan, msg.ID, err.Error())
					// 清理会话
					if session, ok := sessions[msg.ID]; ok {
						session.Writer.Close()
						delete(sessions, msg.ID)
					}
				}
			case wsMsgTypeEnd:
				// 上传结束
				err := handleEndUpload(msgChan, &msg, sessions)
				if err != nil {
					slog.Error("处理上传结束失败", slog.Any("err", err))
					sendError(msgChan, msg.ID, err.Error())
				}
				// 清理会话
				if session, ok := sessions[msg.ID]; ok {
					session.Writer.Close()
					delete(sessions, msg.ID)
				}
			default:
				sendError(msgChan, msg.ID, fmt.Sprintf("unknown message type: %s", msg.Type))
			}
		}

		// 清理所有会话
		for _, session := range sessions {
			session.Writer.Close()
		}

		// 关闭消息队列，等待发送 goroutine 完成
		close(msgChan)
		<-done
	}
}

// handleStartUpload 处理开始上传
func handleStartUpload(msgChan chan<- WebSocketUploadMessage, msg *WebSocketUploadMessage, fd finder.Finder, sessions map[string]*WebSocketUploadSession) error {
	// 创建流式写入器
	writer, err := createUploadWriter(fd, msg.Path, msg.FileName)
	if err != nil {
		return fmt.Errorf("创建上传流失败: %w", err)
	}

	session := &WebSocketUploadSession{
		ID:       msg.ID,
		FileName: msg.FileName,
		Path:     msg.Path,
		Size:     msg.Size,
		Writer:   writer,
		Finder:   fd,
		Offset:   0,
	}

	sessions[msg.ID] = session

	// 设置进度回调，通过 WebSocket 发送进度更新
	// 注意：需要在 Close() 之前设置，因为 Close() 会触发 SFTP 写入
	if uploadWriter, ok := writer.(*uploadWriter); ok {
		uploadWriter.SetProgressCallback(func(written, total int64) {
			// 通过消息队列发送 SFTP 写入进度更新（非阻塞）
			select {
			case msgChan <- WebSocketUploadMessage{
				Type:        wsMsgTypeProgress,
				ID:          msg.ID,
				Size:        total,
				Offset:      session.Offset, // 接收进度
				SFTPWritten: written,        // SFTP 写入进度
			}:
			default:
				// 如果队列满了，记录警告但不阻塞
				slog.Warn("消息队列已满，跳过进度更新")
			}
		})
	}

	slog.Info("开始上传文件",
		slog.String("id", msg.ID),
		slog.String("fileName", msg.FileName),
		slog.String("path", msg.Path),
		slog.Int64("size", msg.Size),
	)

	// 发送确认消息
	msgChan <- WebSocketUploadMessage{
		Type:   wsMsgTypeProgress,
		ID:     msg.ID,
		Size:   msg.Size,
		Offset: 0,
	}
	return nil
}

// handleChunk 处理文件数据块
func handleChunk(msgChan chan<- WebSocketUploadMessage, msg *WebSocketUploadMessage, sessions map[string]*WebSocketUploadSession) error {
	session, ok := sessions[msg.ID]
	if !ok {
		return fmt.Errorf("session not found: %s", msg.ID)
	}

	// 解码 base64 数据
	// msg.Data 是 json.RawMessage，需要先解析为字符串
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
	session.Offset += int64(n)

	// 获取已写入 SFTP 的字节数（实时进度）
	var sftpWritten int64
	if writer, ok := session.Writer.(*uploadWriter); ok {
		sftpWritten = writer.GetSFTPWritten()
	}

	// 计算进度百分比
	percent := float64(session.Offset) / float64(session.Size) * 100
	sftpPercent := float64(sftpWritten) / float64(session.Size) * 100

	// 每 10% 或每 1MB 或完成时记录一次日志，避免日志过多
	lastLoggedPercent := int(percent/10) * 10
	shouldLog := session.Offset%(1024*1024) == 0 || // 每 1MB
		(session.Offset == session.Size) || // 完成时
		(lastLoggedPercent != session.lastLoggedPercent) // 每 10% 变化时

	if shouldLog {
		session.lastLoggedPercent = lastLoggedPercent
		slog.Info("上传进度",
			slog.String("id", msg.ID),
			slog.String("fileName", session.FileName),
			slog.Int64("offset", session.Offset),
			slog.Int64("size", session.Size),
			slog.Float64("percent", percent),
			slog.Int64("sftpWritten", sftpWritten),
			slog.Float64("sftpPercent", sftpPercent),
			slog.Int("chunkSize", len(chunkData)),
		)
	}

	// 发送进度更新，包含 SFTP 写入进度
	msgChan <- WebSocketUploadMessage{
		Type:        wsMsgTypeProgress,
		ID:          msg.ID,
		Size:        session.Size,
		Offset:      session.Offset,
		SFTPWritten: sftpWritten,
	}
	return nil
}

// handleEndUpload 处理上传结束
func handleEndUpload(msgChan chan<- WebSocketUploadMessage, msg *WebSocketUploadMessage, sessions map[string]*WebSocketUploadSession) error {
	session, ok := sessions[msg.ID]
	if !ok {
		return fmt.Errorf("session not found: %s", msg.ID)
	}

	slog.Info("上传完成，正在关闭写入器",
		slog.String("id", msg.ID),
		slog.String("fileName", session.FileName),
		slog.Int64("totalSize", session.Offset),
	)

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
		slog.Int64("size", session.Offset),
		slog.Int64("sftpWriteDuration", sftpWriteDuration),
	)

	// 获取文件列表
	storage, err := session.Finder.Index(context.Background(), session.Path)
	if err != nil {
		return fmt.Errorf("获取文件列表失败: %w", err)
	}

	// 发送成功消息，包含 SFTP 写入时间信息
	successData, _ := json.Marshal(storage)
	msgChan <- WebSocketUploadMessage{
		Type:              wsMsgTypeSuccess,
		ID:                msg.ID,
		Data:              successData,
		SFTPWriteStart:    sftpWriteStart,
		SFTPWriteEnd:      sftpWriteEnd,
		SFTPWriteDuration: sftpWriteDuration,
	}
	return nil
}

// sendMessage 通过消息队列发送消息
func sendMessage(msgChan chan<- WebSocketUploadMessage, msg WebSocketUploadMessage) error {
	select {
	case msgChan <- msg:
		return nil
	default:
		return fmt.Errorf("消息队列已满")
	}
}

// sendError 通过消息队列发送错误消息
func sendError(msgChan chan<- WebSocketUploadMessage, id, errorMsg string) {
	msgChan <- WebSocketUploadMessage{
		Type:  wsMsgTypeError,
		ID:    id,
		Error: errorMsg,
	}
}

// sendErrorSync 同步发送错误消息（用于初始化阶段，消息队列还未创建时）
func sendErrorSync(conn *websocket.Conn, id, errorMsg string) {
	conn.WriteJSON(WebSocketUploadMessage{
		Type:  wsMsgTypeError,
		ID:    id,
		Error: errorMsg,
	})
}
