package wal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultMaxFileSize 默认单文件最大大小 (100MB)
	DefaultMaxFileSize = 100 * 1024 * 1024
	// DefaultRollingInterval 默认滚动时间间隔 (5 分钟)
	DefaultRollingInterval = 5 * time.Minute
	// DefaultMaxFiles 默认保留的最大文件数
	DefaultMaxFiles = 10
	// KeyCacheDuration 密钥缓存时间 (5 分钟)
	KeyCacheDuration = 5 * time.Minute
	// MaxKeyCacheSize 最大密钥缓存数量 (防止长期运行内存增长)
	MaxKeyCacheSize = 100
	// FileExtension WAL 文件扩展名
	FileExtension = ".wal.enc"
)

// WALEncryptor WAL 加密器
// 使用 AES-256-GCM 加密模式
// P1-5 修复：统一使用单一锁 (mu)，避免密钥状态不一致
type WALEncryptor struct {
	mu sync.RWMutex

	// 当前密钥
	currentKey []byte
	// 密钥版本
	keyVersion uint32

	// 密钥缓存（P1-5 修复：不再使用单独的 cacheMu，统一由 mu 保护）
	keyCache     map[uint32][]byte
	keyCacheTime time.Time

	// K8s Secret 路径 (可选)
	secretPath string
	// 密钥加载函数
	keyLoader KeyLoader
}

// KeyLoader 密钥加载接口
type KeyLoader interface {
	LoadKey() ([]byte, uint32, error)
}

// K8sSecretLoader 从 K8s Secret 加载密钥
type K8sSecretLoader struct {
	secretPath string
}

// LoadKey 从 K8s Secret 加载密钥
func (k *K8sSecretLoader) LoadKey() ([]byte, uint32, error) {
	// 读取密钥文件
	keyPath := filepath.Join(k.secretPath, "key")
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read key from secret: %w", err)
	}

	// 读取密钥版本
	versionPath := filepath.Join(k.secretPath, "version")
	versionData, err := os.ReadFile(versionPath)
	if err != nil {
		// 如果没有版本文件，默认为版本 1
		versionData = []byte("1")
	}

	var version uint32
	if len(versionData) > 0 {
		fmt.Sscanf(string(versionData), "%d", &version)
	}
	if version == 0 {
		version = 1
	}

	// 确保密钥长度为 32 字节 (AES-256)
	if len(keyData) < 32 {
		return nil, 0, fmt.Errorf("key must be at least 32 bytes")
	}

	key := make([]byte, 32)
	copy(key, keyData[:32])

	return key, version, nil
}

// WALWriter WAL 写入器 (支持滚动)
type WALWriter struct {
	mu sync.Mutex

	// 基础目录
	baseDir string
	// 当前文件
	currentFile *os.File
	// 当前文件名
	currentName string
	// 当前文件大小
	currentSize int64
	// 当前文件创建时间
	currentTime time.Time

	// 加密器
	encryptor *WALEncryptor

	// 配置
	maxFileSize     int64
	rollingInterval time.Duration
	maxFiles        int

	// 当前文件序号
	currentIndex uint64
}

// WALReader WAL 读取器
type WALReader struct {
	mu sync.Mutex

	// 基础目录
	baseDir string
	// 加密器
	encryptor *WALEncryptor
	// 已打开的文件
	files map[string]*os.File
}

// NewWALEncryptor 创建 WAL 加密器
func NewWALEncryptor(secretPath string) (*WALEncryptor, error) {
	e := &WALEncryptor{
		secretPath: secretPath,
		keyCache:   make(map[uint32][]byte),
	}

	// 如果有 secretPath，使用 K8s Secret Loader
	if secretPath != "" {
		e.keyLoader = &K8sSecretLoader{secretPath: secretPath}
		// 立即加载密钥
		key, version, err := e.keyLoader.LoadKey()
		if err != nil {
			return nil, fmt.Errorf("failed to load initial key: %w", err)
		}
		e.currentKey = key
		e.keyVersion = version
		e.keyCache[version] = key
		e.keyCacheTime = time.Now()
	} else {
		// 否则生成随机密钥 (仅用于测试)
		log.Printf("WARNING: TEST MODE - Using random key, data will be lost on restart")
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("failed to generate random key: %w", err)
		}
		e.currentKey = key
		e.keyVersion = 1
		e.keyCache[1] = key
		e.keyCacheTime = time.Now()
	}

	return e, nil
}

// RefreshKey 刷新密钥 (检查缓存是否过期)
// P1-5 修复：统一使用单一锁 (mu)，避免密钥状态不一致
func (e *WALEncryptor) RefreshKey() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查缓存是否过期
	if time.Since(e.keyCacheTime) < KeyCacheDuration {
		return nil
	}

	// 缓存过期，重新加载
	if e.keyLoader != nil {
		key, version, err := e.keyLoader.LoadKey()
		if err != nil {
			return err
		}

		// P1-5 修复：所有密钥相关操作都在同一个锁保护下
		e.currentKey = key
		e.keyVersion = version
		e.keyCache[version] = key
		e.keyCacheTime = time.Now()
	}

	return nil
}

// GetCurrentKey 获取当前密钥
func (e *WALEncryptor) GetCurrentKey() ([]byte, uint32) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentKey, e.keyVersion
}

// GetKeyByVersion 根据版本获取密钥
// P1-5 修复：统一使用单一锁 (mu)
func (e *WALEncryptor) GetKeyByVersion(version uint32) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	key, ok := e.keyCache[version]
	if !ok {
		return nil, fmt.Errorf("key version %d not found", version)
	}

	return key, nil
}

// Encrypt 加密数据
// 格式：[4 字节长度][4 字节版本][密文 (nonce+ 加密数据)]
func (e *WALEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	e.mu.RLock()
	key := e.currentKey
	keyVersion := e.keyVersion
	e.mu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// 格式：[4 字节长度][4 字节版本][密文]
	totalLen := 4 + 4 + len(ciphertext)
	result := make([]byte, totalLen)
	binary.BigEndian.PutUint32(result[:4], uint32(4+len(ciphertext))) // 版本 + 密文的长度
	binary.BigEndian.PutUint32(result[4:8], keyVersion)
	copy(result[8:], ciphertext)

	return result, nil
}

// Decrypt 解密数据
// 格式：[4 字节长度][4 字节版本][密文 (nonce+ 加密数据)]
func (e *WALEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 8 {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// 读取总长度
	totalLen := binary.BigEndian.Uint32(ciphertext[:4])
	// 读取密钥版本
	keyVersion := binary.BigEndian.Uint32(ciphertext[4:8])

	// 获取对应版本的密钥
	key, err := e.GetKeyByVersion(keyVersion)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 解密数据 (跳过 8 字节前缀)
	nonceSize := gcm.NonceSize()
	data := ciphertext[8:]

	cipherLen := int(totalLen) - 4 // 密文长度 = totalLen - 4(版本)
	if len(data) < cipherLen {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := data[:nonceSize]
	plaintext, err := gcm.Open(nil, nonce, data[nonceSize:cipherLen], nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// RotateKey 轮换密钥
func (e *WALEncryptor) RotateKey() error {
	// 生成新密钥
	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return fmt.Errorf("failed to generate new key: %w", err)
	}

	e.mu.Lock()
	e.keyVersion++
	e.currentKey = newKey
	e.keyCache[e.keyVersion] = newKey
	e.keyCacheTime = time.Now()
	e.mu.Unlock()

	return nil
}

// NewWALWriter 创建 WAL 写入器
func NewWALWriter(baseDir string, encryptor *WALEncryptor) (*WALWriter, error) {
	return NewWALWriterWithConfig(baseDir, encryptor, DefaultMaxFileSize, DefaultRollingInterval, DefaultMaxFiles)
}

// NewWALWriterWithConfig 创建 WAL 写入器 (自定义配置)
func NewWALWriterWithConfig(baseDir string, encryptor *WALEncryptor, maxFileSize int64, rollingInterval time.Duration, maxFiles int) (*WALWriter, error) {
	// 创建目录
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	w := &WALWriter{
		baseDir:         baseDir,
		encryptor:       encryptor,
		maxFileSize:     maxFileSize,
		rollingInterval: rollingInterval,
		maxFiles:        maxFiles,
	}

	// 查找现有文件，确定下一个序号
	files, err := w.getWALFiles()
	if err != nil {
		return nil, err
	}

	if len(files) > 0 {
		// 使用最后一个文件的序号 +1
		w.currentIndex = files[len(files)-1].index + 1
	} else {
		w.currentIndex = 1
	}

	// 打开或创建当前文件
	if err := w.openCurrentFile(); err != nil {
		return nil, err
	}

	return w, nil
}

// walFileInfo WAL 文件信息
type walFileInfo struct {
	name    string
	index   uint64
	modTime time.Time
	size    int64
}

// getWALFiles 获取所有 WAL 文件 (按序号排序)
func (w *WALWriter) getWALFiles() ([]walFileInfo, error) {
	entries, err := os.ReadDir(w.baseDir)
	if err != nil {
		return nil, err
	}

	var files []walFileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, FileExtension) {
			continue
		}

		// 解析序号 (000001.wal.enc -> 1)
		var index uint64
		fmt.Sscanf(name, "%06d", &index)

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, walFileInfo{
			name:    name,
			index:   index,
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}

	// 按序号排序
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].index > files[j].index {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	return files, nil
}

// openCurrentFile 打开或创建当前文件
func (w *WALWriter) openCurrentFile() error {
	filename := fmt.Sprintf("%06d%s", w.currentIndex, FileExtension)
	filepath := filepath.Join(w.baseDir, filename)

	// 检查文件是否存在
	info, err := os.Stat(filepath)
	if err == nil {
		// 文件存在，打开并获取大小
		file, err := os.OpenFile(filepath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		w.currentFile = file
		w.currentName = filename
		w.currentSize = info.Size()
		w.currentTime = info.ModTime()
		return nil
	}

	// 文件不存在，创建新文件
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}

	w.currentFile = file
	w.currentName = filename
	w.currentSize = 0
	w.currentTime = time.Now()

	return nil
}

// Write 写入数据 (自动处理滚动)
// P1-4 修复：确保锁覆盖整个滚动过程，防止竞态条件导致数据丢失
func (w *WALWriter) Write(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 加密数据
	encrypted, err := w.encryptor.Encrypt(data)
	if err != nil {
		return err
	}

	// 检查是否需要滚动
	needRoll := false

	// 按大小滚动
	if w.currentSize+int64(len(encrypted)) > w.maxFileSize {
		needRoll = true
	}

	// 按时间滚动
	if time.Since(w.currentTime) > w.rollingInterval {
		needRoll = true
	}

	if needRoll {
		// P1-4 修复：rollFile 必须在锁保护下执行
		// 关闭当前文件、创建新文件、清理旧文件都必须是原子的
		if err := w.rollFileLocked(); err != nil {
			return err
		}
	}

	// 写入数据
	n, err := w.currentFile.Write(encrypted)
	if err != nil {
		return err
	}

	w.currentSize += int64(n)

	return nil
}

// rollFile 滚动文件（公共方法，获取自己的锁）
func (w *WALWriter) rollFile() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rollFileLocked()
}

// rollFileLocked 滚动文件（内部方法，假设调用者已持有锁）
// P1-4 修复：确保整个滚动过程在锁保护下原子执行
func (w *WALWriter) rollFileLocked() error {
	// 关闭当前文件
	if w.currentFile != nil {
		if err := w.currentFile.Close(); err != nil {
			return err
		}
	}

	// 递增序号
	w.currentIndex++

	// 创建新文件
	if err := w.openCurrentFile(); err != nil {
		return err
	}

	// 清理旧文件
	if err := w.cleanupOldFiles(); err != nil {
		return err
	}

	return nil
}

// cleanupOldFiles 清理旧文件 (保留最近 N 个)
func (w *WALWriter) cleanupOldFiles() error {
	files, err := w.getWALFiles()
	if err != nil {
		return err
	}

	// 如果文件数超过限制，删除最旧的文件
	for len(files) > w.maxFiles {
		oldest := files[0]
		filepath := filepath.Join(w.baseDir, oldest.name)

		if err := os.Remove(filepath); err != nil {
			return err
		}

		files = files[1:]
	}

	return nil
}

// Close 关闭写入器
func (w *WALWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentFile != nil {
		return w.currentFile.Close()
	}

	return nil
}

// NewWALReader 创建 WAL 读取器
func NewWALReader(baseDir string, encryptor *WALEncryptor) (*WALReader, error) {
	return &WALReader{
		baseDir:   baseDir,
		encryptor: encryptor,
		files:     make(map[string]*os.File),
	}, nil
}

// ReadAll 读取所有数据
func (r *WALReader) ReadAll() ([][]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 获取所有 WAL 文件
	files, err := r.getWALFiles()
	if err != nil {
		return nil, err
	}

	var results [][]byte

	// 按顺序读取所有文件
	for _, file := range files {
		data, err := r.readFile(file.name)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file.name, err)
		}
		results = append(results, data...)
	}

	return results, nil
}

// readFile 读取单个文件
func (r *WALReader) readFile(filename string) ([][]byte, error) {
	filepath := filepath.Join(r.baseDir, filename)

	// 读取文件内容
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	// 解密所有记录
	var records [][]byte
	offset := 0

	for offset < len(data) {
		// 读取总长度 (4 字节)
		if offset+4 > len(data) {
			break
		}
		totalLen := binary.BigEndian.Uint32(data[offset : offset+4])

		// 读取完整记录 (长度 + 版本 + 密文)
		recordLen := 4 + int(totalLen)
		if offset+recordLen > len(data) {
			break
		}

		recordData := data[offset : offset+recordLen]
		offset += recordLen

		// 解密
		plaintext, err := r.encryptor.Decrypt(recordData)
		if err != nil {
			// 解密失败，可能是最后一条记录不完整，停止
			break
		}

		records = append(records, plaintext)
	}

	return records, nil
}

// getWALFiles 获取所有 WAL 文件
func (r *WALReader) getWALFiles() ([]walFileInfo, error) {
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		return nil, err
	}

	var files []walFileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, FileExtension) {
			continue
		}

		// 解析序号
		var index uint64
		fmt.Sscanf(name, "%06d", &index)

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, walFileInfo{
			name:    name,
			index:   index,
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}

	// 按序号排序
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].index > files[j].index {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	return files, nil
}

// Close 关闭读取器
func (r *WALReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, file := range r.files {
		if err := file.Close(); err != nil {
			return err
		}
	}

	return nil
}
