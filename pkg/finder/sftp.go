package finder

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/ecodeclub/ekit/slice"
	"github.com/pkg/sftp"
)

const (
	storageName = "sftp"
	pathPrefix  = "sftp://"
)

type sftpFinder struct {
	client *sftp.Client
}

func NewSftpFinder(client *sftp.Client) Finder {
	return &sftpFinder{
		client: client,
	}
}

func (sf *sftpFinder) Save(ctx context.Context, path, content string) error {
	// 解析路径为实际文件系统路径
	actualPath := parseVueFinderPath(path)

	// 使用 os.O_WRONLY|os.O_TRUNC 来覆盖文件内容
	file, err := sf.client.OpenFile(actualPath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE)
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

func (sf *sftpFinder) Preview(ctx context.Context, path string) (bytes.Buffer, error) {
	// 解析路径为实际文件系统路径
	actualPath := parseVueFinderPath(path)

	var buff bytes.Buffer
	file, err := sf.client.Open(actualPath)
	if err != nil {
		return buff, err
	}

	if _, err = file.WriteTo(&buff); err != nil {
		return buff, err
	}

	return buff, nil
}

func (sf *sftpFinder) Search(ctx context.Context, adapter, path, filter string) (Storages, error) {
	storage, err := sf.Index(ctx, path)
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
	// 解析路径为实际文件系统路径
	actualTarget := parseVueFinderPath(target)
	actualBase := parseVueFinderPath(base)

	// 判断是否有后缀，如果没有自行添加上
	zipFileName := ensureZipExtension(actualTarget)

	zipFile, err := sf.client.Create(zipFileName)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, item := range items {
		actualItemPath := parseVueFinderPath(item.Path)
		if blockOperation("archive", actualBase, actualItemPath) {
			continue
		}

		err = sf.walkAndZip(actualItemPath, zipWriter, actualBase)
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
	// 解析目标路径为实际文件系统路径
	actualTarget := parseVueFinderPath(target)

	for _, item := range items {
		// 解析源路径为实际文件系统路径
		actualItemPath := parseVueFinderPath(item.Path)
		fileName := filepath.Base(actualItemPath)
		destPath := filepath.Join(actualTarget, fileName)
		destPath = normalizePath(destPath)

		err := sf.client.Rename(actualItemPath, destPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sf *sftpFinder) Delete(ctx context.Context, items []Item, path string) error {
	// 解析路径为实际文件系统路径
	actualPath := parseVueFinderPath(path)

	for _, item := range items {
		// 解析项目路径为实际文件系统路径
		actualItemPath := parseVueFinderPath(item.Path)
		if blockOperation("remove", actualPath, actualItemPath) {
			continue
		}

		switch item.Type {
		case DIR:
			err := sf.RemoveDir(ctx, actualItemPath)
			if err != nil {
				return err
			}
		case FILE:
			err := sf.RemoveFile(ctx, actualItemPath)
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
	// 解析路径为实际文件系统路径
	actualOldPath := parseVueFinderPath(oldPathName)
	newPath := replaceLastPart(actualOldPath, newName)
	newPath = normalizePath(newPath)

	if blockOperation("rename", actualOldPath, newPath) {
		return nil
	}

	return sf.client.Rename(actualOldPath, newPath)
}

func replaceLastPart(originalPath, newName string) string {
	// 获取路径的父目录
	parentDir := filepath.Dir(originalPath)

	// 拼接新的路径
	newPath := filepath.Join(parentDir, newName)
	return newPath
}

func (sf *sftpFinder) NewFolder(ctx context.Context, file string, name string) error {
	// 解析路径为实际文件系统路径
	actualPath := parseVueFinderPath(file)
	path := filepath.Join(actualPath, name)
	path = normalizePath(path)
	return sf.client.MkdirAll(path)
}

func (sf *sftpFinder) NewFile(ctx context.Context, file string, name string) error {
	// 解析路径为实际文件系统路径
	actualPath := parseVueFinderPath(file)
	path := filepath.Join(actualPath, name)
	path = normalizePath(path)
	_, err := sf.client.Create(path)
	return err
}

func (sf *sftpFinder) Download(ctx context.Context, filePath string) (bytes.Buffer, error) {
	// 解析路径为实际文件系统路径
	actualPath := parseVueFinderPath(filePath)

	var buff bytes.Buffer
	file, err := sf.client.Open(actualPath)
	if err != nil {
		return buff, err
	}

	if _, err = file.WriteTo(&buff); err != nil {
		return buff, err
	}

	return buff, nil
}

func (sf *sftpFinder) Upload(ctx context.Context, src *multipart.FileHeader, remoteDir, remoteFile string) error {
	// 解析路径为实际文件系统路径
	actualRemoteDir := parseVueFinderPath(remoteDir)

	// 如果 remoteFile 包含路径分隔符，需要解析出目录和文件名
	if strings.Contains(remoteFile, "/") {
		parts := strings.Split(remoteFile, "/")
		remoteFile = parts[len(parts)-1]
		actualRemoteDir = filepath.Join(actualRemoteDir, strings.Join(parts[:len(parts)-1], "/"))
	}

	// 构建完整的目标文件路径
	targetPath := filepath.Join(actualRemoteDir, remoteFile)
	targetPath = normalizePath(targetPath)

	// 确保目标目录存在
	targetDir := filepath.Dir(targetPath)
	if _, err := sf.client.Stat(targetDir); os.IsNotExist(err) {
		if err = sf.client.MkdirAll(targetDir); err != nil {
			return err
		}
	}

	// 打开源文件
	srcFile, err := src.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 创建并打开目标文件
	dstFile, err := sf.client.Create(targetPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// 缓冲区读取并写入 4MB
	buffer := make([]byte, 4*1024*1024)
	for {
		n, readErr := srcFile.Read(buffer)
		if n > 0 {
			if _, writeErr := dstFile.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	return nil
}

// parseVueFinderPath 解析 vuefinder 格式路径 (sftp://path 或 tmp://) 为实际文件系统路径
func parseVueFinderPath(path string) string {
	if !strings.Contains(path, "://") {
		// 如果不是 vuefinder 格式，直接规范化返回
		return normalizePath(path)
	}

	parts := strings.SplitN(path, "://", 2)
	if len(parts) < 2 {
		return "/"
	}

	// parts[0] 可能是适配器名称（如 sftp）或根目录名称（如 tmp）
	// parts[1] 是实际路径
	if parts[1] == "" {
		// 如果 parts[1] 为空，检查 parts[0] 是否是根目录名称
		// 例如：tmp:// -> /tmp
		if parts[0] != "sftp" && parts[0] != "" {
			// 可能是根目录名称，如 tmp:// -> /tmp
			return normalizePath("/" + parts[0])
		}
		return "/"
	}

	return normalizePath(parts[1])
}

// formatVueFinderPath 将实际文件系统路径格式化为 vuefinder 格式 (sftp://path)
func formatVueFinderPath(path string) string {
	if strings.HasPrefix(path, pathPrefix) {
		return path
	}
	return pathPrefix + normalizePath(path)
}

// normalizePath 规范化路径，确保以 / 开头且没有多余的斜杠
func normalizePath(path string) string {
	path = strings.TrimLeft(path, "/")
	if path == "" {
		return "/"
	}
	return "/" + path
}

func (sf *sftpFinder) Index(ctx context.Context, path string) (Storages, error) {
	// 确定实际路径和显示路径
	actualPath, dirName := sf.resolvePath(path)

	// storages 始终返回所有根目录列表，用于左侧导航栏显示
	// 前端会根据当前路径自动展开/折叠对应的目录树
	storages, err := sf.findStorage()
	if err != nil {
		return Storages{}, err
	}

	// 扫描文件列表（只包含当前路径下的文件和文件夹）
	files, err := sf.scanFiles(actualPath)
	if err != nil {
		return Storages{}, err
	}

	return Storages{
		Storages: storages,
		Dirname:  dirName,
		Files:    files,
	}, nil
}

// resolvePath 解析路径，返回实际文件系统路径和 vuefinder 格式路径
func (sf *sftpFinder) resolvePath(path string) (actualPath, dirName string) {
	if path == "" {
		// 如果路径为空，使用当前工作目录
		pwd, err := sf.client.Getwd()
		if err != nil {
			pwd = "/"
		}
		actualPath = normalizePath(pwd)
		dirName = formatVueFinderPath(actualPath)
		return
	}

	// 解析 vuefinder 格式路径
	actualPath = parseVueFinderPath(path)
	dirName = formatVueFinderPath(actualPath)
	return
}

func (sf *sftpFinder) findStorage() ([]string, error) {
	fileInfos, err := sf.client.ReadDir("/")
	if err != nil {
		return nil, err
	}

	return slice.FilterMap(fileInfos, func(idx int, src os.FileInfo) (string, bool) {
		return src.Name(), src.IsDir() || src.Mode()&os.ModeSymlink != 0
	}), nil
}

// ScanFiles 查找指定路径下所有文件
func (sf *sftpFinder) scan(path string) ([]FileInfo, error) {
	files, err := sf.client.ReadDir(path)
	if err != nil {
		return nil, err
	}

	fileInfos := make([]FileInfo, 0)

	for _, file := range files {
		f := convertToFileInfo(file, path)
		fileInfos = append(fileInfos, f)
	}

	return fileInfos, nil
}

// convertToFileInfo 将 os.FileInfo 转换为 FileInfo
func convertToFileInfo(file os.FileInfo, basePath string) FileInfo {
	ext := strings.TrimPrefix(filepath.Ext(file.Name()), ".")
	mimeType := mime.TypeByExtension("." + ext)

	// 构建文件路径并格式化为 vuefinder 格式
	filePath := filepath.Join(basePath, file.Name())
	filePath = normalizePath(filePath)
	filePath = formatVueFinderPath(filePath)

	// 确定文件类型
	fileType := determineFileType(file, basePath)

	return FileInfo{
		Type:          fileType,
		Path:          filePath,
		Visibility:    "public",
		LastModified:  file.ModTime().Unix(),
		MimeType:      mimeType,
		ExtraMetadata: []string{},
		Basename:      file.Name(),
		Extension:     ext,
		Storage:       storageName,
		FileSize:      file.Size(),
	}
}

// determineFileType 确定文件类型
func determineFileType(file os.FileInfo, basePath string) FileType {
	if file.IsDir() {
		return DIR
	}

	if file.Mode()&os.ModeSymlink != 0 {
		fileType, err := getLinkType(file, basePath)
		if err != nil {
			slog.Error("Failed to get link type",
				"file", file.Name(),
				"err", err)
			return BrokenLINK
		}
		return fileType
	}

	return FILE
}

// getLinkType 获取软链接的目标类型
func getLinkType(file os.FileInfo, basePath string) (FileType, error) {
	// 构建完整的符号链接路径
	linkPath := filepath.Join(basePath, file.Name())

	// 首先检查符号链接文件本身是否存在
	if _, err := os.Lstat(linkPath); err != nil {
		if os.IsNotExist(err) {
			return BrokenLINK, nil
		}
		return "", fmt.Errorf("lstat link %s: %w", linkPath, err)
	}

	// 读取符号链接目标
	targetPath, err := os.Readlink(linkPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 这个错误应该不会发生，因为上面已经检查过了
			return BrokenLINK, nil
		}
		return "", fmt.Errorf("readlink %s: %w", linkPath, err)
	}

	// 如果目标是相对路径，转换为绝对路径
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(filepath.Dir(linkPath), targetPath)
	}

	// 获取目标的文件信息
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return BrokenLINK, nil
		}
		return "", fmt.Errorf("stat target %s: %w", targetPath, err)
	}

	if targetInfo.IsDir() {
		return DIR, nil
	}
	return FILE, nil
}

func (sf *sftpFinder) scanFiles(path string) ([]FileInfo, error) {
	fileInfos := make([]FileInfo, 0)

	// 如果不是根目录，添加 "." 和 ".." 目录
	if !isRootPath(path) {
		parentPath := filepath.Dir(path)
		fileInfos = append(fileInfos,
			createDotDir(path),
			createDotDotDir(parentPath),
		)
	}

	// 扫描目录下的文件
	files, err := sf.scan(path)
	if err != nil {
		return nil, err
	}

	fileInfos = append(fileInfos, files...)
	return fileInfos, nil
}

// createDotDir 创建当前目录 "." 的 FileInfo
func createDotDir(path string) FileInfo {
	return FileInfo{
		Basename: ".",
		Type:     DIR,
		Path:     formatVueFinderPath(path),
		Storage:  storageName,
	}
}

// createDotDotDir 创建父目录 ".." 的 FileInfo
func createDotDotDir(parentPath string) FileInfo {
	return FileInfo{
		Basename: "..",
		Type:     DIR,
		Path:     formatVueFinderPath(parentPath),
		Storage:  storageName,
	}
}

// isRootPath 判断是否为根路径
func isRootPath(path string) bool {
	return strings.Count(path, "/") == 1
}

func blockOperation(action string, oldFile, newFile string) bool {
	// 根据路径长度，避免删除或修改 .. 上级目录 这种情况
	if len(oldFile) > len(newFile) {
		slog.Error("发现触发危险操作, 被系统阻止", slog.String("当前", oldFile), slog.String("处理", newFile), slog.String("动作", action))
		return true
	}

	return false
}
