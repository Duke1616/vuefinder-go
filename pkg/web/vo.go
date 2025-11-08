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
	Items []Item `json:"items"`
}

type MoveReq struct {
	Path  string `json:"path"`
	Item  string `json:"item"`
	Items []Item `json:"items"`
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
