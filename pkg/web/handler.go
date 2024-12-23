package web

import (
	"errors"
	"github.com/Duke1616/vuefinder-go/pkg/finder"
	"github.com/Duke1616/vuefinder-go/pkg/ginx"
	"github.com/ecodeclub/ekit/slice"
	"github.com/gin-gonic/gin"
	"net/http"
	"path"
	"strconv"
)

type Handler struct {
	finders map[int64]finder.Finder
}

func NewHandler() *Handler {
	return &Handler{
		finders: make(map[int64]finder.Finder),
	}
}

func (h *Handler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/api/finder")

	g.GET("/index", ginx.Wrap(h.Index))
	g.GET("/subfolders", ginx.Wrap(h.Subfolders))
	g.GET("/download", ginx.WrapData(h.Download))
	g.GET("/search", ginx.Wrap(h.Search))
	g.GET("/preview", ginx.WrapBuff(h.Preview))
	g.POST("/upload", ginx.Wrap(h.Upload))
	g.POST("/new_folder", ginx.WrapBody(h.NewFolder))
	g.POST("/new_file", ginx.WrapBody(h.NewFile))
	g.POST("/rename", ginx.WrapBody(h.Rename))
	g.POST("/remove", ginx.WrapBody(h.Remove))
	g.POST("/move", ginx.WrapBody(h.Move))
	g.POST("/archive", ginx.WrapBody(h.Archive))
	g.POST("/save", ginx.WrapBuffBody(h.Save))
}

func (h *Handler) SetFinder(id int64, f finder.Finder) {
	h.finders[id] = f
}

func (h *Handler) getFinder(ctx *gin.Context) (finder.Finder, error) {
	queryId := ctx.Query("id")
	id, err := strconv.ParseInt(queryId, 10, 64)
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
	pathQuery := ctx.Query("path")
	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Save(ctx, pathQuery, req.Content)
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

	// 根据文件类型设置响应的 Content-Type
	contentType := http.DetectContentType(buff.Bytes())
	ctx.Header("Content-Type", contentType)

	return ginx.Result{Message: "OK", Data: buff.String()}, nil
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
	pathQuery := ctx.Query("path")
	adapter := ctx.Query("adapter")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}
	err = fd.Archive(ctx, toFinderItems(req.Items), req.Name, pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, adapter, pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) Move(ctx *gin.Context, req MoveReq) (ginx.Result, error) {
	pathQuery := ctx.Query("path")
	adapter := ctx.Query("adapter")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Move(ctx, toFinderItems(req.Items), req.Item)

	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, adapter, pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) Remove(ctx *gin.Context, req RemoveReq) (ginx.Result, error) {
	pathQuery := ctx.Query("path")
	adapter := ctx.Query("adapter")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Remove(ctx, toFinderItems(req.Items), pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, adapter, pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) Rename(ctx *gin.Context, req RenameReq) (ginx.Result, error) {
	pathQuery := ctx.Query("path")
	adapter := ctx.Query("adapter")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Rename(ctx, req.Item, req.Name, pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, adapter, pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) NewFile(ctx *gin.Context, req NewFileReq) (ginx.Result, error) {
	pathQuery := ctx.Query("path")
	adapter := ctx.Query("adapter")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.NewFile(ctx, pathQuery, req.Name)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, adapter, pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: storage,
	}, nil
}

func (h *Handler) NewFolder(ctx *gin.Context, req NewFolderReq) (ginx.Result, error) {
	pathQuery := ctx.Query("path")
	adapter := ctx.Query("adapter")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.NewFolder(ctx, pathQuery, req.Name)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	storage, err := fd.Index(ctx, adapter, pathQuery)
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

	ctx.Header("Content-Description", "File Transfer")
	ctx.Header("Content-Transfer-Encoding", "binary")
	ctx.Header("Content-Disposition", "attachment; filename="+path.Base(file))
	ctx.Header("Content-Type", "application/octet-stream")

	return ginx.Result{
		Data: buff.Bytes(),
	}, nil
}

func (h *Handler) Subfolders(ctx *gin.Context) (ginx.Result, error) {
	pathQuery := ctx.Query("path")
	adapter := ctx.Query("adapter")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	files, err := fd.Subfolders(ctx, adapter, pathQuery)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{
		Data: &RetrieveFolder{
			Folders: files,
		},
	}, nil
}

func (h *Handler) Upload(ctx *gin.Context) (ginx.Result, error) {
	file, err := ctx.FormFile("file")
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	src, err := file.Open()
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	// 文件名称
	remoteFile, _ := ctx.GetPostForm("name")
	remoteDir := ctx.Query("path")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	err = fd.Upload(ctx, src, remoteDir, remoteFile)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	return ginx.Result{}, nil
}

func (h *Handler) Index(ctx *gin.Context) (ginx.Result, error) {
	adapterQuery := ctx.Query("adapter")
	pathQuery := ctx.Query("path")

	fd, err := h.getFinder(ctx)
	if err != nil {
		return ginx.Result{Message: err.Error()}, err
	}

	data, err := fd.Index(ctx, adapterQuery, pathQuery)
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
