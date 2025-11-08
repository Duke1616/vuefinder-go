package web

import (
	"bytes"

	"github.com/Duke1616/vuefinder-go/pkg/finder"
)

type NewFolderReq struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type NewFileReq struct {
	Path string `bson:"path"`
	Name string `json:"name"`
}

type DeleteReq struct {
	Items []Item `json:"items"`
	Path  string `json:"path"`
}

type UploadReq struct {
	Name string       `json:"name"`
	Type string       `json:"type"`
	Path string       `json:"path"`
	File bytes.Buffer `json:"file"`
}

type RenameReq struct {
	Path string `json:"path"`
	Item string `json:"item"`
	Name string `json:"name"`
}

type ArchiveReq struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Items []Item `json:"items"`
}

type UnarchiveReq struct {
	Item string `json:"item"` // 要解压的文件路径
	Path string `json:"path"` // 解压到的目标目录
}

type MoveReq struct {
	Path        string   `json:"path"`
	Item        string   `json:"item"`        // 目标路径（destination）
	Items       []Item   `json:"items"`       // 源文件列表（sources）
	Destination string   `json:"destination"` // 目标路径（前端可能使用这个字段）
	Sources     []string `json:"sources"`     // 源文件列表（前端发送的是字符串数组）
}

type SaveReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type Item struct {
	Path string          `json:"path"`
	Type finder.FileType `json:"type"`
}

type RetrieveFolder struct {
	Folders []finder.FileInfo `json:"folders"`
}
