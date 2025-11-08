package ginx

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func Wrap(fn func(ctx *gin.Context) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := fn(ctx)
		if err != nil {
			slog.Error("执行业务逻辑失败", slog.Any("err", err))
			// 确保错误返回格式统一：code=0 表示失败
			errorRes := Result{
				Code:    0,
				Message: res.Message,
				Data:    nil,
			}
			if errorRes.Message == "" {
				errorRes.Message = err.Error()
			}
			ctx.PureJSON(http.StatusInternalServerError, errorRes)
			return
		}
		ctx.PureJSON(http.StatusOK, res.Data)
	}
}

func WrapBuffBody[Req any](fn func(ctx *gin.Context, req Req) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req Req
		if err := ctx.Bind(&req); err != nil {
			slog.Error("绑定参数失败", slog.Any("err", err))
			ctx.PureJSON(http.StatusBadRequest, Result{
				Code:    0,
				Message: err.Error(),
				Data:    nil,
			})
			return
		}

		res, err := fn(ctx, req)
		if err != nil {
			slog.Error("执行业务逻辑失败", slog.Any("err", err))
			errorRes := Result{
				Code:    0,
				Message: res.Message,
				Data:    nil,
			}
			if errorRes.Message == "" {
				errorRes.Message = err.Error()
			}
			ctx.PureJSON(http.StatusInternalServerError, errorRes)
			return
		}
		ctx.String(http.StatusOK, "%s", res.Data)
	}
}

func WrapBuff(fn func(ctx *gin.Context) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := fn(ctx)
		if err != nil {
			slog.Error("执行业务逻辑失败", slog.Any("err", err))
			errorRes := Result{
				Code:    0,
				Message: res.Message,
				Data:    nil,
			}
			if errorRes.Message == "" {
				errorRes.Message = err.Error()
			}
			ctx.PureJSON(http.StatusInternalServerError, errorRes)
			return
		}
		ctx.String(http.StatusOK, "%s", res.Data)
	}
}

func WrapData(fn func(ctx *gin.Context) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := fn(ctx)
		if err != nil {
			slog.Error("执行业务逻辑失败", slog.Any("err", err))
			errorRes := Result{
				Code:    0,
				Message: res.Message,
				Data:    nil,
			}
			if errorRes.Message == "" {
				errorRes.Message = err.Error()
			}
			ctx.PureJSON(http.StatusInternalServerError, errorRes)
			return
		}

		// 将 res.Data 转换为 []byte 类型
		data, ok := res.Data.([]byte)
		if !ok {
			slog.Error("res.Data 不是 []byte 类型")
			ctx.PureJSON(http.StatusInternalServerError, Result{
				Code:    0,
				Message: "无法处理返回的数据",
				Data:    nil,
			})
			return
		}

		// 发送二进制数据
		ctx.Data(http.StatusOK, "application/octet-stream", data)
	}
}

func WrapBody[Req any](fn func(ctx *gin.Context, req Req) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req Req
		if err := ctx.Bind(&req); err != nil {
			slog.Error("绑定参数失败", slog.Any("err", err))
			ctx.PureJSON(http.StatusBadRequest, Result{
				Code:    0,
				Message: err.Error(),
				Data:    nil,
			})
			return
		}

		res, err := fn(ctx, req)
		if err != nil {
			slog.Error("执行业务逻辑失败", slog.Any("err", err))
			errorRes := Result{
				Code:    0,
				Message: res.Message,
				Data:    nil,
			}
			if errorRes.Message == "" {
				errorRes.Message = err.Error()
			}
			ctx.PureJSON(http.StatusInternalServerError, errorRes)
			return
		}
		ctx.PureJSON(http.StatusOK, res.Data)
	}
}

// WrapUpload 专门用于文件上传的包装函数，确保响应格式正确
// 注意：上传进度由浏览器的 XMLHttpRequest 自动处理，不需要后端特殊处理
// uppy 期望服务器返回 JSON 格式的响应，可以是任何有效的 JSON 对象
func WrapUpload(fn func(ctx *gin.Context) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := fn(ctx)
		if err != nil {
			slog.Error("上传文件失败", slog.Any("err", err))
			errorRes := Result{
				Code:    0,
				Message: res.Message,
				Data:    nil,
			}
			if errorRes.Message == "" {
				errorRes.Message = err.Error()
			}
			ctx.PureJSON(http.StatusInternalServerError, errorRes)
			return
		}
		// 返回完整的 Result 对象
		// uppy 会解析响应，但上传进度是由浏览器自动报告的，不依赖响应内容
		ctx.PureJSON(http.StatusOK, res)
	}
}
