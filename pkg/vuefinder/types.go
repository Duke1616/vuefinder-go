package finderx

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
	Index(ctx context.Context, adapter, path string) (*FinderStorages, error)
	Upload(ctx context.Context, file multipart.File, remoteDir, remoteFile string) error
	Download(ctx context.Context, filePath string) (bytes.Buffer, error)
	Rename(ctx context.Context, oldName, newName string) error
	NewFolder(ctx context.Context, file string) error
	NewFile(ctx context.Context, file string) error
	RemoveDir(ctx context.Context, file string) error
	RemoveFile(ctx context.Context, file string) error
	Archive(ctx context.Context, files []string, target, base string) error
	Move(ctx context.Context, files []string, target string) error
	Preview(ctx context.Context, path string) (bytes.Buffer, error)
	Search(ctx context.Context, adapter, path, filter string) (*FinderStorages, error)
	Subfolders(ctx context.Context, adapter, path string) ([]FileInfo, error)
}

type FinderStorages struct {
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
