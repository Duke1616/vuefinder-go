package web

import "github.com/Duke1616/vuefinder-go/pkg/finder"

type NewFolderReq struct {
	Name string `json:"name"`
}

type NewFileReq struct {
	Name string `json:"name"`
}

type RenameReq struct {
	Item string `json:"item"`
	Name string `json:"name"`
}

type ArchiveReq struct {
	Name  string `json:"name"`
	Items []Item `json:"items"`
}

type MoveReq struct {
	Item  string `json:"item"`
	Items []Item `json:"items"`
}

type SaveReq struct {
	Content string `json:"content"`
}

type RemoveReq struct {
	Items []Item `json:"items"`
}

type Item struct {
	Path string          `json:"path"`
	Type finder.FileType `json:"type"`
}

type RetrieveFolder struct {
	Folders []finder.FileInfo `json:"folders"`
}
