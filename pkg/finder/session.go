package finder

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/sftp"
)

type sftpFinder struct {
	client *sftp.Client
}

func (s *sftpUploadSession) FinalPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finalPath
}

// sftpUploadSession 基于远端 .part 文件的随机写会话
type sftpUploadSession struct {
	client    *sftp.Client
	partPath  string
	finalPath string
	f         *sftp.File
	mu        sync.Mutex
	closed    bool
}

func (s *sftpUploadSession) ensureOpen() error {
	if s.f != nil {
		return nil
	}
	// 以只写方式打开或创建 .part 文件
	f, err := s.client.OpenFile(s.partPath, os.O_WRONLY|os.O_CREATE)
	if err != nil {
		return err
	}
	s.f = f
	return nil
}

func (s *sftpUploadSession) WriteAt(p []byte, off int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	if err := s.ensureOpen(); err != nil {
		return 0, err
	}
	// 尝试写入，失败后重开一次再试
	n, err := s.f.WriteAt(p, off)
	if err == nil {
		return n, nil
	}
	// 重试一次
	_ = s.f.Close()
	s.f = nil
	if e := s.ensureOpen(); e != nil {
		return 0, err
	}
	return s.f.WriteAt(p, off)
}

func (s *sftpUploadSession) Size() (int64, error) {
	info, err := s.client.Stat(s.partPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return info.Size(), nil
}

func (s *sftpUploadSession) Commit() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && s.f != nil {
		_ = s.f.Close()
		s.f = nil
		s.closed = true
	}
	// 原子重命名 .part -> 最终文件
	// 若最终文件已存在，则生成不冲突的新文件名（如 name (1).ext）
	// 检查目标是否存在
	if _, statErr := s.client.Stat(s.finalPath); statErr == nil {
		dir := filepath.Dir(s.finalPath)
		base := filepath.Base(s.finalPath)
		name := base
		ext := ""
		if dot := strings.LastIndex(base, "."); dot > 0 {
			name = base[:dot]
			ext = base[dot:]
		}
		// 依次尝试 name (1).ext, name (2).ext, ...
		for i := 1; i < 10000; i++ {
			cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
			if _, err := s.client.Stat(cand); err != nil { // 不存在则可用
				s.finalPath = cand
				break
			}
		}
	}
	return s.client.Rename(s.partPath, s.finalPath)
}

func (s *sftpUploadSession) Abort() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && s.f != nil {
		_ = s.f.Close()
		s.f = nil
		s.closed = true
	}
	// 删除 .part 文件
	_ = s.client.Remove(s.partPath)
	return nil
}

func (s *sftpUploadSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.f != nil {
		err := s.f.Close()
		s.f = nil
		return err
	}
	return nil
}
