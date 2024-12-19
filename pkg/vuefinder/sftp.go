package finderx

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"github.com/ecodeclub/ekit/slice"
	"github.com/pkg/sftp"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

type sftpFinder struct {
	user   string
	client *sftp.Client
}

func NewSftpFinder(client *sftp.Client, user string) Finder {
	return &sftpFinder{
		client: client,
		user:   user,
	}
}

func (sf *sftpFinder) Subfolders(ctx context.Context, adapter, path string) ([]FileInfo, error) {
	files, err := sf.scan(path, adapter)
	if err != nil {
		return nil, err
	}

	files = slice.FilterMap(files, func(idx int, src FileInfo) (FileInfo, bool) {
		if src.Type != DIR {
			return src, false
		}

		return src, true
	})

	return files, nil
}

func (sf *sftpFinder) Preview(ctx context.Context, path string) (bytes.Buffer, error) {
	var buff bytes.Buffer
	file, err := sf.client.Open(path)
	if err != nil {
		return buff, err
	}

	if _, err = file.WriteTo(&buff); err != nil {
		return buff, err
	}

	return buff, nil
}

func (sf *sftpFinder) Search(ctx context.Context, adapter, path, filter string) (*FinderStorages, error) {
	storage, err := sf.Index(ctx, adapter, path)
	if err != nil {
		return nil, err
	}

	storage.Files = slice.FilterMap(storage.Files, func(idx int, src FileInfo) (FileInfo, bool) {
		if strings.Contains(src.Basename, filter) {
			return src, true
		}
		return FileInfo{}, false
	})

	return storage, nil
}

func ensureZipExtension(target string) string {
	// 检查目标路径的扩展名是否为 .zip
	if !strings.HasSuffix(target, ".zip") {
		// 如果没有 .zip 后缀，则添加 .zip
		target = target + ".zip"
	}
	return target
}

func (sf *sftpFinder) Archive(ctx context.Context, files []string, target, base string) error {
	// 判断是否有后缀，如果没有自行添加上
	zipFileName := ensureZipExtension(ensureZipExtension(target))

	zipFile, err := sf.client.Create(zipFileName)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, file := range files {
		err = sf.walkAndZip(file, zipWriter, base)
		if err != nil {
			return fmt.Errorf("failed to walk and zip %s: %w", file, err)
		}
	}

	return nil
}

func (sf *sftpFinder) walkAndZip(path string, zipWriter *zip.Writer, basePath string) error {
	info, err := sf.client.Stat(path)
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	// 去掉公共前缀
	relativePath := strings.TrimPrefix(path, basePath)
	header.Name = relativePath
	if info.IsDir() {
		header.Name += "/"
		header.Method = zip.Store
	} else {
		header.Method = zip.Deflate
	}

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		var remoteFile *sftp.File
		remoteFile, err = sf.client.Open(path)
		if err != nil {
			return err
		}
		defer remoteFile.Close()

		_, err = io.Copy(writer, remoteFile)
		if err != nil {
			return err
		}
	}

	if info.IsDir() {
		var files []os.FileInfo
		files, err = sf.client.ReadDir(path)
		if err != nil {
			return err
		}

		for _, file := range files {
			subPath := filepath.Join(path, file.Name())
			err = sf.walkAndZip(subPath, zipWriter, basePath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (sf *sftpFinder) Move(ctx context.Context, files []string, target string) error {
	for _, file := range files {
		fileName := filepath.Base(file)
		destPath := filepath.Join(target, fileName)

		err := sf.client.Rename(file, destPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sf *sftpFinder) RemoveDir(ctx context.Context, file string) error {
	return sf.client.RemoveAll(file)
}

func (sf *sftpFinder) RemoveFile(ctx context.Context, file string) error {
	return sf.client.Remove(file)
}

func (sf *sftpFinder) Rename(ctx context.Context, oldName, newName string) error {
	return sf.client.Rename(oldName, newName)
}

func (sf *sftpFinder) NewFolder(ctx context.Context, file string) error {
	return sf.client.MkdirAll(file)
}

func (sf *sftpFinder) NewFile(ctx context.Context, file string) error {
	_, err := sf.client.Create(file)
	return err
}

func (sf *sftpFinder) Download(ctx context.Context, filePath string) (bytes.Buffer, error) {
	var buff bytes.Buffer
	file, err := sf.client.Open(filePath)
	if err != nil {
		return buff, err
	}

	if _, err = file.WriteTo(&buff); err != nil {
		return buff, err
	}

	return buff, nil
}

func (sf *sftpFinder) Upload(ctx context.Context, file multipart.File, remoteDir, remoteFile string) error {
	if _, err := sf.client.Stat(remoteDir); os.IsNotExist(err) {
		if err = sf.client.MkdirAll(remoteDir); err != nil {
			return err
		}
	}

	dstFile, err := sf.client.Create(remoteFile)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err = io.Copy(dstFile, file); err != nil {
		return err
	}

	return nil
}

func (sf *sftpFinder) Index(ctx context.Context, adapter, path string) (*FinderStorages, error) {
	var (
		storages   []string
		files      []FileInfo
		err        error
		dirName    string
		newAdapter string
	)

	// 获取跟目录数据
	if storages, err = sf.findStorage(); err != nil {
		return nil, err
	}

	// 如果是第一次请求，则针对用户加目录
	if adapter != "null" {
		newAdapter = adapter
		dirName = getPath(newAdapter, path)
		if files, err = sf.scanFiles(dirName, newAdapter); err != nil {
			return nil, err
		}

	} else {
		newAdapter = "home"
		dirName = fmt.Sprintf("/home/%s", sf.user)
		if files, err = sf.scanFiles(fmt.Sprintf("/home/%s", sf.user), newAdapter); err != nil {
			return nil, err
		}
	}

	return &FinderStorages{
		Adapter:  newAdapter,
		Storages: storages,
		Dirname:  dirName,
		Files:    files,
	}, nil
}

func (sf *sftpFinder) findStorage() ([]string, error) {
	fileInfos, err := sf.client.ReadDir("/")
	if err != nil {
		return nil, err
	}

	var storages []string
	for _, file := range fileInfos {
		// 不是目录同时也不是软连接文件直接退出，默认跟目录不允许存储文件
		if !file.IsDir() && file.Mode()&os.ModeSymlink == 0 {
			continue
		}

		storages = append(storages, file.Name())
	}

	return storages, nil
}

// ScanFiles 查找指定路径下所有文件
func (sf *sftpFinder) scan(path, adapter string) ([]FileInfo, error) {
	files, err := sf.client.ReadDir(path)
	if err != nil {
		return nil, err
	}

	fileInfos := make([]FileInfo, 0)

	for _, file := range files {
		f := convertToFileInfo(file, path, adapter)
		fileInfos = append(fileInfos, f)
	}

	return fileInfos, nil
}

// convertToFileInfo 文件转换为finder前端识别
func convertToFileInfo(file os.FileInfo, path, adapter string) FileInfo {
	ext := strings.TrimPrefix(filepath.Ext(file.Name()), ".")
	mimeType := mime.TypeByExtension("." + ext)

	// 构建 FileInfo 结构体
	return FileInfo{
		Type: func() FileType {
			if file.IsDir() {
				return DIR
			} else if file.Mode()&os.ModeSymlink != 0 {
				fileType, err := getLinkType(file, adapter)
				if err != nil {
					slog.Error("Failed to get link type", file.Name(), slog.Any("err", err))
				}
				return fileType
			} else {
				return FILE
			}
		}(),
		Path:          fmt.Sprintf("%s/%s", path, file.Name()),
		Visibility:    "public",
		LastModified:  file.ModTime(),
		MimeType:      mimeType,
		ExtraMetadata: []string{},
		Basename:      file.Name(),
		Extension:     ext,
		Storage:       adapter,
		FileSize:      file.Size(),
	}
}

// getLinkType 获取软链接的目标类型
func getLinkType(file os.FileInfo, path string) (FileType, error) {
	// 读取目标文件
	targetPath, err := os.Readlink(filepath.Join(path, file.Name()))
	if err != nil {
		return "", err
	}

	// 获取目标的文件信息
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return "", err
	}

	// 判断是否为目录
	if targetInfo.IsDir() {
		return DIR, nil
	} else {
		return FILE, nil
	}
}

func (sf *sftpFinder) scanFiles(path, adapter string) ([]FileInfo, error) {
	fileInfos := make([]FileInfo, 0)
	// 不是跟目录的情况下进行添
	if matchPath(path) {
		fileInfos = append(fileInfos, FileInfo{
			Basename: ".",
			Type:     DIR,
			Path:     path,
		})
		fileInfos = append(fileInfos, FileInfo{
			Basename: "..",
			Type:     DIR,
			Path:     filepath.Dir(path),
		})
	}

	files, err := sf.scan(path, adapter)
	fileInfos = append(fileInfos, files...)
	return fileInfos, err
}

func getPath(adapter, path string) string {
	if path == "" {
		return fmt.Sprintf("/%s", adapter)
	}

	// 点击导航栏搜索框跳转
	if strings.Contains(path, ":///") {
		parts := strings.SplitN(path, ":///", 2)
		return fmt.Sprintf("/%s", parts[1])
	}

	return path
}

// matchPath 判断路径是否已经在 / 或 /home 这样的路径下
func matchPath(path string) bool {
	// 计算路径中 / 的数量
	count := strings.Count(path, "/")

	// 只有一个 / 不匹配
	return count != 1
}
