package sftp

import "github.com/Duke1616/vuefinder-go/pkg/provider"

var _ provider.Readable = (*sftpFinder)(nil)
var _ provider.ResumableUploader = (*sftpFinder)(nil)
var _ provider.Writer = (*sftpFinder)(nil)
var _ provider.Lister = (*sftpFinder)(nil)
var _ provider.Searcher = (*sftpFinder)(nil)
var _ provider.Previewer = (*sftpFinder)(nil)
