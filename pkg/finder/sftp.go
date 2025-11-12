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
	"sync"
	"time"

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

func (sf *sftpFinder) Unarchive(ctx context.Context, archivePath, targetDir string) error {
	// 解析路径为实际文件系统路径
	actualArchivePath := parseVueFinderPath(archivePath)
	actualTargetDir := parseVueFinderPath(targetDir)

	// 打开 ZIP 文件
	zipFile, err := sf.client.Open(actualArchivePath)
	if err != nil {
		return fmt.Errorf("打开压缩文件失败: %w", err)
	}
	defer zipFile.Close()

	// 获取文件大小
	fileInfo, err := zipFile.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 读取 ZIP 文件内容到内存
	zipData := make([]byte, fileInfo.Size())
	_, err = io.ReadFull(zipFile, zipData)
	if err != nil {
		return fmt.Errorf("读取压缩文件失败: %w", err)
	}

	// 创建 ZIP Reader
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), fileInfo.Size())
	if err != nil {
		return fmt.Errorf("解析压缩文件失败: %w", err)
	}

	// 遍历 ZIP 文件中的每个条目
	for _, file := range zipReader.File {
		// 清理文件名：移除 Windows 路径分隔符，防止路径遍历
		cleanName := strings.ReplaceAll(file.Name, "\\", "/")
		cleanName = strings.TrimPrefix(cleanName, "/")

		// 安全检查：防止路径遍历攻击
		if strings.Contains(cleanName, "..") {
			continue
		}

		// 构建目标文件路径
		targetPath := filepath.Join(actualTargetDir, cleanName)
		targetPath = normalizePath(targetPath)

		// 如果是目录
		if file.FileInfo().IsDir() {
			err = sf.client.MkdirAll(targetPath)
			if err != nil {
				return fmt.Errorf("创建目录失败 %s: %w", targetPath, err)
			}
			continue
		}

		// 确保父目录存在
		parentDir := filepath.Dir(targetPath)
		err = sf.client.MkdirAll(parentDir)
		if err != nil {
			return fmt.Errorf("创建父目录失败 %s: %w", parentDir, err)
		}

		// 打开 ZIP 文件中的文件
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("打开压缩文件中的文件失败 %s: %w", file.Name, err)
		}

		// 创建目标文件
		targetFile, err := sf.client.Create(targetPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("创建目标文件失败 %s: %w", targetPath, err)
		}

		// 复制文件内容
		_, err = io.Copy(targetFile, rc)
		rc.Close()
		targetFile.Close()

		if err != nil {
			return fmt.Errorf("写入文件失败 %s: %w", targetPath, err)
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
			return fmt.Errorf("failed to move %s to %s: %w", actualItemPath, destPath, err)
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

// Open 打开远程文件并返回可随机读取的视图，用于流式下载/断点续传
func (sf *sftpFinder) Open(ctx context.Context, filePath string) (ra io.ReaderAt, size int64, modTime time.Time, name string, closer io.Closer, err error) {
    // 解析路径为实际文件系统路径
    actualPath := parseVueFinderPath(filePath)

    // 打开文件
    f, e := sf.client.Open(actualPath)
    if e != nil {
        err = e
        return
    }

    // 获取文件信息
    info, e := f.Stat()
    if e != nil {
        f.Close()
        err = e
        return
    }

    // 返回 ReaderAt（*sftp.File 实现了 ReadAt）、大小、修改时间、名称和关闭器
    ra = f
    size = info.Size()
    modTime = info.ModTime()
    name = info.Name()
    closer = f
    return
}

func (sf *sftpFinder) Upload(ctx context.Context, src *multipart.FileHeader, remoteDir, remoteFile string) error {
	// 打开源文件
	srcFile, err := src.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// 使用流式上传
	return sf.UploadStream(ctx, srcFile, remoteDir, remoteFile)
}

// UploadStream 流式上传文件，边接收边写入，不等待整个文件上传完成
func (sf *sftpFinder) UploadStream(ctx context.Context, src io.Reader, remoteDir, remoteFile string) error {
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

	// 创建并打开目标文件
	dstFile, err := sf.client.Create(targetPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// 使用 io.CopyBuffer 进行流式复制，缓冲区大小为 256KB
	// 256KB 是 32KB (SFTP MaxPacketSize) 的 8 倍，可以减少系统调用次数
	// 同时不会占用太多内存，是一个较好的平衡点
	buffer := make([]byte, 256*1024)

	// 直接复制，不调用 Sync() 以提高性能
	// Sync() 会强制刷新到磁盘，但会显著降低性能
	// 文件关闭时会自动刷新，不需要手动 Sync
	_, err = io.CopyBuffer(dstFile, src, buffer)
	if err != nil {
		return err
	}

	return nil
}

// UploadStreamWithProgress 流式上传文件，支持进度回调
func (sf *sftpFinder) UploadStreamWithProgress(ctx context.Context, src io.Reader, remoteDir, remoteFile string,
	totalSize int64, onProgress func(written, total int64)) error {
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

	// 创建并打开目标文件
	dstFile, err := sf.client.Create(targetPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// 创建进度跟踪写入器
	// 每 64KB 报告一次进度，提供更细粒度的进度更新
	progressWriter := &progressWriter{
		writer:         dstFile,
		total:          totalSize,
		onProgress:     onProgress,
		reportInterval: 64 * 1024, // 64KB 报告间隔，提供更频繁的进度更新
	}

	// 使用自定义的 Copy 函数，在每次写入后都检查是否需要报告进度
	// 缓冲区大小为 256KB，但进度报告间隔为 64KB
	buffer := make([]byte, 256*1024)
	_, err = copyWithProgress(progressWriter, src, buffer)
	if err != nil {
		return err
	}

	// 确保最后报告 100% 进度
	if onProgress != nil {
		onProgress(totalSize, totalSize)
	}

	return nil
}

// progressWriter 包装 io.Writer，跟踪写入进度并通过回调发送
type progressWriter struct {
	writer         io.Writer
	total          int64
	written        int64
	onProgress     func(written, total int64) // 进度回调函数
	lastReported   int64                      // 上次报告的字节数
	reportInterval int64                      // 报告间隔（字节），避免过于频繁的回调
	mu             sync.Mutex
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.writer.Write(p)
	if err != nil {
		return n, err
	}

	pw.mu.Lock()
	pw.written += int64(n)
	written := pw.written
	total := pw.total
	lastReported := pw.lastReported
	reportInterval := pw.reportInterval
	pw.mu.Unlock()

	// 调用进度回调（每 64KB 或完成时报告一次）
	shouldReport := (written-lastReported) >= reportInterval || written >= total
	if pw.onProgress != nil && shouldReport {
		pw.onProgress(written, total)
		pw.mu.Lock()
		pw.lastReported = written
		pw.mu.Unlock()
	}

	return n, err
}

// copyWithProgress 自定义的 Copy 函数，确保进度回调被正确触发
// 使用较小的块大小来读取，以便更频繁地触发进度回调
func copyWithProgress(dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	// 使用较小的读取块（64KB），以便更频繁地触发进度更新
	readBuf := make([]byte, 64*1024)
	if len(buf) > 0 {
		readBuf = buf[:min(len(buf), 64*1024)]
	}

	for {
		nr, er := src.Read(readBuf)
		if nr > 0 {
			nw, ew := dst.Write(readBuf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
		// 如果已经是 sftp:// 格式，需要规范化路径部分
		// 例如：sftp:///var -> sftp:///var, sftp:/// -> sftp:///
		pathPart := strings.TrimPrefix(path, pathPrefix)
		normalized := normalizePath(pathPart)
		return pathPrefix + normalized
	}
	// normalizePath 已经确保路径以 / 开头，所以直接拼接
	normalized := normalizePath(path)
	return pathPrefix + normalized
}

// normalizePath 规范化路径，确保以 / 开头且没有多余的斜杠
func normalizePath(path string) string {
	// 移除开头的所有斜杠
	path = strings.TrimLeft(path, "/")
	if path == "" {
		return "/"
	}
	// 移除路径中多余的连续斜杠，并确保以 / 开头
	path = strings.ReplaceAll(path, "//", "/")
	return "/" + path
}

func (sf *sftpFinder) Index(ctx context.Context, path string) (Storages, error) {
	// 确定实际路径和显示路径
	actualPath, dirName := sf.resolvePath(path)

	// storages 始终返回所有根目录列表，用于左侧导航栏显示
	// vuefinder 4.0 的左侧导航栏会遍历 storages 数组，显示每个根目录
	// 当点击某个根目录时，前端会调用 list(storage + "://") 来获取该目录下的内容
	// 每次 list 调用后，前端会调用 setStorages 更新 storages，所以需要始终返回所有根目录列表
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

	// 获取文件所在目录路径（Dir 字段）
	dirPath := normalizePath(basePath)
	dirPath = formatVueFinderPath(dirPath)

	// 确定文件类型
	fileType := determineFileType(file, basePath)

	return FileInfo{
		Type:          fileType,
		Dir:           dirPath,
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

	// 扫描目录下的文件
	files, err := sf.scan(path)
	if err != nil {
		return nil, err
	}

	// 过滤掉 "." 和 ".." 目录，避免前端渲染时出现循环引用
	filteredFiles := make([]FileInfo, 0, len(files))
	for _, file := range files {
		if file.Basename != "." && file.Basename != ".." {
			filteredFiles = append(filteredFiles, file)
		}
	}

	fileInfos = append(fileInfos, filteredFiles...)

	return fileInfos, nil
}

func blockOperation(action string, oldFile, newFile string) bool {
	// 根据路径长度，避免删除或修改 .. 上级目录 这种情况
	if len(oldFile) > len(newFile) {
		slog.Error("发现触发危险操作, 被系统阻止", slog.String("当前", oldFile), slog.String("处理", newFile), slog.String("动作", action))
		return true
	}

	return false
}
