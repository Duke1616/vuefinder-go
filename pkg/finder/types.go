package finder

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
)

type FileType string

const (
	DIR        FileType = "dir"
	FILE       FileType = "file"
	LINK       FileType = "link"
	BrokenLINK FileType = "broken-link"
)

type Finder interface {
	Index(ctx context.Context, path string) (Storages, error)
	Upload(ctx context.Context, src *multipart.FileHeader, remoteDir, remoteFile string) error
	UploadStream(ctx context.Context, src io.Reader, remoteDir, remoteFile string) error
	Download(ctx context.Context, filePath string) (bytes.Buffer, error)
	Rename(ctx context.Context, oldPathName, newName, path string) error
	NewFolder(ctx context.Context, file, name string) error
	NewFile(ctx context.Context, file, name string) error
	RemoveDir(ctx context.Context, file string) error
	RemoveFile(ctx context.Context, file string) error
	Archive(ctx context.Context, items []Item, target, base string) error
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
