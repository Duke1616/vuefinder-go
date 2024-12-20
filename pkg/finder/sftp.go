package finder

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

func (sf *sftpFinder) Save(ctx context.Context, path, content string) error {
	// 使用 os.O_WRONLY|os.O_TRUNC 来覆盖文件内容
	file, err := sf.client.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入内容到文件
	_, err = file.Write([]byte(content))
	if err != nil {
		return err
	}

	return nil
}

func (sf *sftpFinder) Subfolders(ctx context.Context, adapter, path string) ([]FileInfo, error) {
	if strings.Contains(path, "://") {
		split := strings.Split(path, "://")
		if split[1] == "" {
			path = fmt.Sprintf("/%s", adapter)
		} else {
			path = split[1]
		}
	}

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

func (sf *sftpFinder) Search(ctx context.Context, adapter, path, filter string) (Storages, error) {
	storage, err := sf.Index(ctx, adapter, path)
	if err != nil {
		return Storages{}, err
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

func (sf *sftpFinder) Archive(ctx context.Context, items []Item, target, base string) error {
	// 判断是否有后缀，如果没有自行添加上
	zipFileName := ensureZipExtension(ensureZipExtension(target))

	zipFile, err := sf.client.Create(zipFileName)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, item := range items {
		if blockOperation("archive", base, item.Path) {
			continue
		}

		err = sf.walkAndZip(item.Path, zipWriter, base)
		if err != nil {
			return err
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

func (sf *sftpFinder) Move(ctx context.Context, items []Item, target string) error {
	for _, item := range items {
		fileName := filepath.Base(item.Path)
		destPath := filepath.Join(target, fileName)

		err := sf.client.Rename(item.Path, destPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sf *sftpFinder) Remove(ctx context.Context, items []Item, path string) error {
	for _, item := range items {
		if blockOperation("remove", path, item.Path) {
			continue
		}

		switch item.Type {
		case DIR:
			err := sf.RemoveDir(ctx, item.Path)
			if err != nil {
				return err
			}
		case FILE:
			err := sf.RemoveFile(ctx, item.Path)
			if err != nil {
				return err
			}
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

func (sf *sftpFinder) Rename(ctx context.Context, oldPathName, newName, path string) error {
	newPath := replaceLastPart(oldPathName, newName)
	if blockOperation("rename", oldPathName, newPath) {
		return nil
	}

	return sf.client.Rename(oldPathName, newPath)
}

func replaceLastPart(originalPath, newName string) string {
	// 获取路径的父目录
	parentDir := filepath.Dir(originalPath)

	// 拼接新的路径
	newPath := filepath.Join(parentDir, newName)
	return newPath
}

func (sf *sftpFinder) NewFolder(ctx context.Context, file string, name string) error {
	return sf.client.MkdirAll(fmt.Sprintf("/%s/%s", file, name))
}

func (sf *sftpFinder) NewFile(ctx context.Context, file string, name string) error {
	_, err := sf.client.Create(fmt.Sprintf("/%s/%s", file, name))
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
	// 如果 remoteFile 包含 "/"，则需要解析出目录和文件名
	if strings.Contains(remoteFile, "/") {
		parts := strings.Split(remoteFile, "/")
		// 最后一个部分是文件名
		remoteFile = parts[len(parts)-1]
		// 前面的部分是目录
		remoteDir = fmt.Sprintf("%s/%s", remoteDir, strings.Join(parts[:len(parts)-1], "/"))
	}

	remoteFile = fmt.Sprintf("%s/%s", remoteDir, remoteFile)

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

func (sf *sftpFinder) Index(ctx context.Context, adapter, path string) (Storages, error) {
	var (
		storages   []string
		files      []FileInfo
		err        error
		dirName    string
		newAdapter string
	)

	// 获取跟目录数据
	if storages, err = sf.findStorage(); err != nil {
		return Storages{}, err
	}

	// 如果是第一次请求，则针对用户加目录
	if adapter != "null" {
		newAdapter = adapter
		dirName = getPath(newAdapter, path)
		if files, err = sf.scanFiles(dirName, newAdapter); err != nil {
			return Storages{}, err
		}

	} else {
		newAdapter = "home"
		dirName = fmt.Sprintf("/home/%s", sf.user)
		if files, err = sf.scanFiles(fmt.Sprintf("/home/%s", sf.user), newAdapter); err != nil {
			return Storages{}, err
		}
	}

	return Storages{
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

func blockOperation(action string, oldFile, newFile string) bool {
	// 根据路径长度，避免删除或修改 .. 上级目录 这种情况
	if len(oldFile) > len(newFile) {
		slog.Error("发现触发危险操作, 被系统阻止", slog.String("当前", oldFile), slog.String("处理", newFile), slog.String("动作", action))
		return true
	}

	return false
}
