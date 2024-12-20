package finder

import (
	"bytes"
	"context"
	"mime/multipart"
	"time"
)

type FileType string

const (
	// DIR 目录
	DIR FileType = "dir"
	// FILE 文件
	FILE FileType = "file"
)

type Finder interface {
	Index(ctx context.Context, adapter, path string) (Storages, error)
	Upload(ctx context.Context, file multipart.File, remoteDir, remoteFile string) error
	Download(ctx context.Context, filePath string) (bytes.Buffer, error)
	Rename(ctx context.Context, oldPathName, newName, path string) error
	NewFolder(ctx context.Context, file, name string) error
	NewFile(ctx context.Context, file, name string) error
	Remove(ctx context.Context, items []Item, path string) error
	RemoveDir(ctx context.Context, file string) error
	RemoveFile(ctx context.Context, file string) error
	Archive(ctx context.Context, items []Item, target, base string) error
	Move(ctx context.Context, items []Item, target string) error
	Preview(ctx context.Context, path string) (bytes.Buffer, error)
	Search(ctx context.Context, adapter, path, filter string) (Storages, error)
	Subfolders(ctx context.Context, adapter, path string) ([]FileInfo, error)
	Save(ctx context.Context, path, content string) error
}

type Storages struct {
	Adapter  string     `json:"adapter"`
	Storages []string   `json:"storages"`
	Dirname  string     `json:"dirname"`
	Files    []FileInfo `json:"files"`
}

type FileInfo struct {
	Type          FileType  `json:"type"`
	Path          string    `json:"path"`
	Visibility    string    `json:"visibility"`
	LastModified  time.Time `json:"last_modified"`
	MimeType      string    `json:"mime_type"`
	ExtraMetadata []string  `json:"extra_metadata"`
	Basename      string    `json:"basename"`
	Extension     string    `json:"extension"`
	Storage       string    `json:"storage"`
	FileSize      int64     `json:"file_size"`
}

type Item struct {
	Path string   `json:"path"`
	Type FileType `json:"type"`
}
