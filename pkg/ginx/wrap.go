package ginx

import (
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
)

func Wrap(fn func(ctx *gin.Context) (Result, error)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := fn(ctx)
		if err != nil {
			slog.Error("执行业务逻辑失败", slog.Any("err", err))
			ctx.PureJSON(http.StatusInternalServerError, res)
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
			return
		}

		res, err := fn(ctx, req)
		if err != nil {
			slog.Error("执行业务逻辑失败", slog.Any("err", err))
			ctx.PureJSON(http.StatusInternalServerError, res)
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
			ctx.PureJSON(http.StatusInternalServerError, res)
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
			ctx.PureJSON(http.StatusInternalServerError, res)
			return
		}

		// 将 res.Data 转换为 []byte 类型
		data, ok := res.Data.([]byte)
		if !ok {
			slog.Error("res.Data 不是 []byte 类型")
			ctx.PureJSON(http.StatusInternalServerError, gin.H{
				"error": "无法处理返回的数据",
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
			return
		}

		res, err := fn(ctx, req)
		if err != nil {
			slog.Error("执行业务逻辑失败", slog.Any("err", err))
			ctx.PureJSON(http.StatusInternalServerError, res)
			return
		}
		ctx.PureJSON(http.StatusOK, res.Data)
	}
}
