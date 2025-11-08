package web

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/Duke1616/vuefinder-go/pkg/finder"
	"github.com/Duke1616/vuefinder-go/pkg/ginx"
	"github.com/ecodeclub/ekit/slice"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	finders map[int64]finder.Finder
}

func NewHandler() *Handler {
	return &Handler{
		finders: make(map[int64]finder.Finder),
	}
}

// RegisterUploadRoute 单独注册上传路由，在中间件之前
// 使用流式上传以支持实时进度显示
func (h *Handler) RegisterUploadRoute(server *gin.Engine) {
	// HTTP 流式上传（保留作为备选）
	server.Any("/api/finder/upload", func(ctx *gin.Context) {
		StreamingUploadHandler(h)(ctx.Writer, ctx.Request)
	})

	// WebSocket 上传（支持实时进度和双向通信）
	server.GET("/api/finder/upload/ws", func(ctx *gin.Context) {
		UploadHandler(h)(ctx.Writer, ctx.Request)
	})
}

func (h *Handler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/finder")

	g.GET("/files", ginx.Wrap(h.Index))
	g.GET("/download", ginx.WrapData(h.Download))
	g.GET("/search", ginx.Wrap(h.Search))
	g.GET("/preview", ginx.WrapBuff(h.Preview))
	// 注意：上传路由已经在 RegisterUploadRoute 中注册（流式上传，支持实时进度）
	g.POST("/new_folder", ginx.WrapBody(h.NewFolder))
	g.POST("/new_file", ginx.WrapBody(h.NewFile))
	g.POST("/rename", ginx.WrapBody(h.Rename))
	g.POST("/move", ginx.WrapBody(h.Move))
	g.POST("/archive", ginx.WrapBody(h.Archive))
	g.POST("/unarchive", ginx.WrapBody(h.Unarchive))
	g.POST("/save", ginx.WrapBuffBody(h.Save))
	g.DELETE("/delete", ginx.WrapBody(h.Delete))
}

func (h *Handler) SetFinder(id int64, f finder.Finder) {
	h.finders[id] = f
}

func (h *Handler) getFinder(ctx *gin.Context) (finder.Finder, error) {
	finderID := ctx.GetHeader("x-finder-id")
	id, err := strconv.ParseInt(finderID, 10, 64)

	if err != nil {
		return nil, err
	}

	fd, ok := h.finders[id]
	if !ok {
		return nil, errors.New("finder not found")
	}

	return fd, nil
}

func (h *Handler) Save(ctx *gin.Context, req SaveReq) (ginx.Result, error) {
	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Save(ctx, req.Path, req.Content)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{Message: "OK", Data: req.Content}, nil
}

func (h *Handler) Preview(ctx *gin.Context) (ginx.Result, error) {
	pathQuery := ctx.Query("path")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	// 获取文件内容
	buff, err := fd.Preview(ctx, pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	// 先获取字节数据，避免多次读取
	data := buff.Bytes()

	// 根据文件类型设置响应的 Content-Type
	contentType := http.DetectContentType(data)
	ctx.Header("Content-Type", contentType)

	return ginx.Result{Message: "OK", Data: string(data)}, nil
}

func (h *Handler) Search(ctx *gin.Context) (ginx.Result, error) {
	pathQuery := ctx.Query("path")
	adapter := ctx.Query("adapter")
	filter := ctx.Query("filter")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storages, err := fd.Search(ctx, adapter, pathQuery, filter)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storages,
	}, nil
}

func (h *Handler) Archive(ctx *gin.Context, req ArchiveReq) (ginx.Result, error) {
	// 使用 req.Path，如果没有则从 query 参数获取
	basePath := req.Path
	if basePath == "" {
		basePath = ctx.Query("path")
	}

	// 构建完整的目标路径：basePath + "/" + name + ".zip"
	// 如果 name 已经包含 .zip 扩展名，则不需要再添加
	targetPath := basePath
	if targetPath != "" && !strings.HasSuffix(targetPath, "/") {
		targetPath += "/"
	}
	targetPath += req.Name
	if !strings.HasSuffix(targetPath, ".zip") {
		targetPath += ".zip"
	}

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Archive(ctx, toFinderItems(req.Items), targetPath, basePath)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, basePath)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) Unarchive(ctx *gin.Context, req UnarchiveReq) (ginx.Result, error) {
	if req.Item == "" {
		return ginx.Result{Message: "item is required"}, fmt.Errorf("item is required")
	}

	if req.Path == "" {
		return ginx.Result{Message: "path is required"}, fmt.Errorf("path is required")
	}

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Unarchive(ctx, req.Item, req.Path)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, req.Path)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) Move(ctx *gin.Context, req MoveReq) (ginx.Result, error) {
	// 兼容前端可能使用的不同字段名
	// 优先使用 sources/destination，如果没有则使用 items/item
	var items []Item
	var target string

	// 如果 sources 是字符串数组，需要转换为 Item 数组
	if len(req.Sources) > 0 {
		items = make([]Item, 0, len(req.Sources))
		for _, sourcePath := range req.Sources {
			// 从路径推断类型（这里简化处理，实际可能需要查询文件系统）
			// 暂时都设置为 FILE，如果需要可以后续优化
			items = append(items, Item{
				Path: sourcePath,
				Type: finder.FILE, // 默认类型，可以根据需要调整
			})
		}
	} else {
		items = req.Items
	}

	if req.Destination != "" {
		target = req.Destination
	} else {
		target = req.Item
	}

	if len(items) == 0 {
		return ginx.Result{Message: "no items to move"}, fmt.Errorf("no items to move")
	}

	if target == "" {
		return ginx.Result{Message: "destination is required"}, fmt.Errorf("destination is required")
	}

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Move(ctx, toFinderItems(items), target)

	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, req.Path)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) Delete(ctx *gin.Context, req DeleteReq) (ginx.Result, error) {
	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Delete(ctx, toFinderItems(req.Items), req.Path)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, req.Path)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) Rename(ctx *gin.Context, req RenameReq) (ginx.Result, error) {
	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Rename(ctx, req.Item, req.Name, req.Path)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, req.Path)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) NewFile(ctx *gin.Context, req NewFileReq) (ginx.Result, error) {
	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.NewFile(ctx, req.Path, req.Name)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, req.Path)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) NewFolder(ctx *gin.Context, req NewFolderReq) (ginx.Result, error) {
	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.NewFolder(ctx, req.Path, req.Name)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, req.Path)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) Download(ctx *gin.Context) (ginx.Result, error) {
	file := ctx.Query("path")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	buff, err := fd.Download(ctx, file)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	// 解析路径获取文件名
	actualPath := file
	if strings.Contains(file, "://") {
		parts := strings.SplitN(file, "://", 2)
		if len(parts) > 1 {
			actualPath = parts[1]
		}
	}
	fileName := path.Base(actualPath)

	// 设置响应头
	ctx.Header("Content-Description", "File Transfer")
	ctx.Header("Content-Transfer-Encoding", "binary")
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	ctx.Header("Content-Type", "application/octet-stream")

	return ginx.Result{
		Data: buff.Bytes(),
	}, nil
}

func (h *Handler) Upload(ctx *gin.Context) (ginx.Result, error) {
	// 直接使用 FormFile 方式
	// 注意：Gin 的 FormFile 会等待整个文件上传完成，但这是 Gin 框架的限制
	// 真正的流式处理需要绕过 Gin 的 multipart 解析，实现起来比较复杂且容易出错
	// 当前实现虽然需要等待文件上传完成，但写入是流式的，不会占用太多内存
	return h.uploadWithFormFile(ctx)
}

// uploadWithFormFile 回退方案：使用 FormFile 方式上传
func (h *Handler) uploadWithFormFile(ctx *gin.Context) (ginx.Result, error) {
	// 文件名称和路径
	remoteFile, _ := ctx.GetPostForm("name")
	remoteDir, _ := ctx.GetPostForm("path")

	// 使用 FormFile 读取文件
	srcFileHeader, err := ctx.FormFile("file")
	if err != nil {
		return ginx.Result{Message: fmt.Sprintf("获取文件失败: %v", err)}, err
	}

	// 如果没有提供文件名，使用上传的文件名
	if remoteFile == "" {
		remoteFile = srcFileHeader.Filename
		if remoteFile == "" {
			remoteFile = "uploaded_file"
		}
	}

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: fmt.Sprintf("获取 Finder 失败: %v", err)}, err
	}

	// 打开文件流进行流式上传
	srcFile, err := srcFileHeader.Open()
	if err != nil {
		return ginx.Result{Message: fmt.Sprintf("打开文件流失败: %v", err)}, err
	}
	defer srcFile.Close()

	// 使用流式上传
	err = fd.UploadStream(ctx, srcFile, remoteDir, remoteFile)
	if err != nil {
		return ginx.Result{Message: fmt.Sprintf("上传文件失败: %v", err)}, err
	}

	// 上传成功后，返回当前目录的文件列表，以便前端刷新
	storage, err := fd.Index(ctx, remoteDir)
	if err != nil {
		return ginx.Result{Message: fmt.Sprintf("获取文件列表失败: %v", err)}, err
	}

	return ginx.Result{
		Message: "File uploaded!",
		Data:    storage,
	}, nil
}

func (h *Handler) Index(ctx *gin.Context) (ginx.Result, error) {
	pathQuery := ctx.Query("path")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	data, err := fd.Index(ctx, pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: data,
	}, nil
}

func toFinderItems(req []Item) []finder.Item {
	return slice.Map(req, func(idx int, src Item) finder.Item {
		return finder.Item{
			Path: src.Path,
			Type: src.Type,
		}
	})
}
