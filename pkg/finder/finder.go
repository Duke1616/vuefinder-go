package finder

type FileType string

const (
	DIR        FileType = "dir"
	FILE       FileType = "file"
	LINK       FileType = "link"
	BrokenLINK FileType = "broken-link"
)

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
