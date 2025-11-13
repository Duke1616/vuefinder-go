package provider

import (
	"bytes"
	"context"
	"io"
	"time"

	finder "github.com/Duke1616/vuefinder-go/pkg/finder"
)

// UploadSession 表示一个可在远端随机写入并可提交/回滚的上传会话
type UploadSession interface {
	// WriteAt 在指定偏移写入数据
	WriteAt(p []byte, off int64) (int, error)
	// Size 返回当前远端文件大小
	Size() (int64, error)
	// Commit 提交上传（例如将 .part 重命名为目标文件）
	Commit() error
	// Abort 取消上传并清理临时文件
	Abort() error
	// Close 关闭底层资源
	Close() error
	// FinalPath 返回最终文件的完整路径（在 Commit 后生效）
	FinalPath() string
}

// Capabilities 组合各能力接口，后端可通过 CapabilityProvider 暴露自身能力
type Capabilities struct {
	Readable          Readable
	Writer            Writer
	Lister            Lister
	Searcher          Searcher
	Previewer         Previewer
	ResumableUploader ResumableUploader
}

// CapabilityProvider 由后端实现，用于一次性提供自身具备的能力
type CapabilityProvider interface {
	Caps() *Capabilities
}

// Lister 提供目录/文件列表能力（与前端列表对齐）
type Lister interface {
	Index(ctx context.Context, path string) (finder.Storages, error)
}

// Searcher 提供搜索能力
type Searcher interface {
	Search(ctx context.Context, adapter, path, filter string) (finder.Storages, error)
}

// Previewer 提供预览能力
type Previewer interface {
	Preview(ctx context.Context, path string) (bytes.Buffer, error)
}

// Writer 聚合所有写入/修改类能力，简化调用方
type Writer interface {
	// UploadStream 上传（基础流式）
	UploadStream(ctx context.Context, src io.Reader, remoteDir, remoteFile string) error
	UploadStreamWithProgress(ctx context.Context, src io.Reader, remoteDir, remoteFile string,
		totalSize int64, onProgress func(written, total int64)) error

	// Rename 变更/创建/移动
	Rename(ctx context.Context, oldPathName, newName, path string) error
	NewFolder(ctx context.Context, file, name string) error
	NewFile(ctx context.Context, file, name string) error
	RemoveDir(ctx context.Context, file string) error
	RemoveFile(ctx context.Context, file string) error
	Move(ctx context.Context, items []finder.Item, target string) error

	// Archive 归档/解压
	Archive(ctx context.Context, items []finder.Item, target, base string) error
	Unarchive(ctx context.Context, archivePath, targetDir string) error
	
	// Save 保存/删除
	Save(ctx context.Context, path, content string) error
	Delete(ctx context.Context, items []finder.Item, path string) error
}

// ResumableUploader 提供断点续传直写能力（可与 Finder 解耦）
type ResumableUploader interface {
	// OpenUpload 打开（或创建）远端临时文件用于断点续传直写
	OpenUpload(ctx context.Context, remoteDir, remoteFile string) (UploadSession, error)
}

// Readable 提供随机读能力（用于下载/Range）
type Readable interface {
	// OpenRead 打开一个可随机读取的文件视图，用于流式下载/断点续传。
	// 返回值：
	//  - ra: 实现了 io.ReaderAt 的读取器
	//  - size: 文件大小（字节）
	//  - modTime: 文件的修改时间
	//  - name: 建议的文件名
	//  - closer: 调用方在读取结束后必须关闭
	OpenRead(ctx context.Context, filePath string) (ra io.ReaderAt, size int64, modTime time.Time, name string, closer io.Closer, err error)
}
