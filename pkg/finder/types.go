package finder

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"time"
)

type FileType string

const (
	DIR        FileType = "dir"
	FILE       FileType = "file"
	LINK       FileType = "link"
	BrokenLINK FileType = "broken-link"
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
}

type Finder interface {
	Index(ctx context.Context, path string) (Storages, error)
	Upload(ctx context.Context, src *multipart.FileHeader, remoteDir, remoteFile string) error
	UploadStream(ctx context.Context, src io.Reader, remoteDir, remoteFile string) error
	UploadStreamWithProgress(ctx context.Context, src io.Reader, remoteDir, remoteFile string,
		totalSize int64, onProgress func(written, total int64)) error
	Download(ctx context.Context, filePath string) (bytes.Buffer, error)
	// Open 打开一个可随机读取的文件视图，用于流式下载/断点续传。
	// 返回值：
	//  - ra: 实现了 io.ReaderAt 的读取器
	//  - size: 文件大小（字节）
	//  - modTime: 文件的修改时间
	//  - name: 建议的文件名
	//  - closer: 调用方在读取结束后必须关闭
	Open(ctx context.Context, filePath string) (ra io.ReaderAt, size int64, modTime time.Time, name string, closer io.Closer, err error)
	// OpenUpload 打开（或创建）远端临时文件用于断点续传直写
	OpenUpload(ctx context.Context, remoteDir, remoteFile string) (UploadSession, error)
	Rename(ctx context.Context, oldPathName, newName, path string) error
	NewFolder(ctx context.Context, file, name string) error
	NewFile(ctx context.Context, file, name string) error
	RemoveDir(ctx context.Context, file string) error
	RemoveFile(ctx context.Context, file string) error
	Archive(ctx context.Context, items []Item, target, base string) error
	Unarchive(ctx context.Context, archivePath, targetDir string) error
	Move(ctx context.Context, items []Item, target string) error
	Preview(ctx context.Context, path string) (bytes.Buffer, error)
	Delete(ctx context.Context, items []Item, path string) error
	Search(ctx context.Context, adapter, path, filter string) (Storages, error)
	Save(ctx context.Context, path, content string) error
}

type Storages struct {
	Storages []string   `json:"storages"`
	Dirname  string     `json:"dirname"`
	Files    []FileInfo `json:"files"`
}

type FileInfo struct {
	Type          FileType `json:"type"`
	Dir           string   `json:"dir"`
	Path          string   `json:"path"`
	Visibility    string   `json:"visibility"`
	LastModified  int64    `json:"last_modified"`
	MimeType      string   `json:"mime_type"`
	ExtraMetadata []string `json:"extra_metadata"`
	Basename      string   `json:"basename"`
	Extension     string   `json:"extension"`
	Storage       string   `json:"storage"`
	FileSize      int64    `json:"file_size"`
	ReadOnly      bool     `json:"read_only,omitempty"`
	PreviewUrl    string   `json:"preview_url,omitempty"`
}

type Item struct {
	Path string   `json:"path"`
	Type FileType `json:"type"`
}

func (f FileType) IsDir() bool {
	return f == DIR
}

func (f FileType) IsFile() bool {
	return f == FILE
}
