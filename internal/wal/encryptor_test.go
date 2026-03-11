package wal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestEncryptorEncryptDecrypt 测试加密解密往返
func TestEncryptorEncryptDecrypt(t *testing.T) {
	// 创建加密器
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// 测试数据
	testData := []byte("Hello, WAL Encryption!")

	// 加密
	encrypted, err := encryptor.Encrypt(testData)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// 验证加密后数据长度增加（版本 + 长度 + nonce+ 密文）
	if len(encrypted) <= len(testData) {
		t.Errorf("Encrypted data should be longer than plaintext")
	}

	// 解密
	decrypted, err := encryptor.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	// 验证解密后数据与原始数据相同
	if string(decrypted) != string(testData) {
		t.Errorf("Decrypted data does not match original: got %s, want %s", string(decrypted), string(testData))
	}

	t.Log("✅ Encrypt/Decrypt test passed")
}

// TestEncryptorKeyRotation 测试密钥轮换
func TestEncryptorKeyRotation(t *testing.T) {
	// 创建加密器
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// 获取初始密钥
	initialKey, initialVersion := encryptor.GetCurrentKey()
	if initialKey == nil {
		t.Fatal("Initial key should not be nil")
	}
	if initialVersion != 1 {
		t.Errorf("Initial key version should be 1, got %d", initialVersion)
	}

	// 加密一些数据
	testData1 := []byte("Data before rotation")
	encrypted1, err := encryptor.Encrypt(testData1)
	if err != nil {
		t.Fatalf("Failed to encrypt before rotation: %v", err)
	}

	// 轮换密钥
	err = encryptor.RotateKey()
	if err != nil {
		t.Fatalf("Failed to rotate key: %v", err)
	}

	// 验证密钥版本增加
	newKey, newVersion := encryptor.GetCurrentKey()
	if newKey == nil {
		t.Fatal("New key should not be nil")
	}
	if newVersion != 2 {
		t.Errorf("New key version should be 2, got %d", newVersion)
	}

	// 验证新旧密钥不同
	if string(newKey) == string(initialKey) {
		t.Error("New key should be different from initial key")
	}

	// 加密新数据
	testData2 := []byte("Data after rotation")
	encrypted2, err := encryptor.Encrypt(testData2)
	if err != nil {
		t.Fatalf("Failed to encrypt after rotation: %v", err)
	}

	// 验证新旧加密数据不同
	if string(encrypted1) == string(encrypted2) {
		t.Error("Encrypted data should be different after key rotation")
	}

	// 验证旧数据仍然可以解密（使用缓存的旧密钥）
	decrypted1, err := encryptor.Decrypt(encrypted1)
	if err != nil {
		t.Fatalf("Failed to decrypt old data: %v", err)
	}
	if string(decrypted1) != string(testData1) {
		t.Errorf("Decrypted old data does not match: got %s, want %s", string(decrypted1), string(testData1))
	}

	// 验证新数据可以解密
	decrypted2, err := encryptor.Decrypt(encrypted2)
	if err != nil {
		t.Fatalf("Failed to decrypt new data: %v", err)
	}
	if string(decrypted2) != string(testData2) {
		t.Errorf("Decrypted new data does not match: got %s, want %s", string(decrypted2), string(testData2))
	}

	t.Log("✅ Key rotation test passed")
}

// TestWALWriterRolling 测试文件滚动
func TestWALWriterRolling(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "wal-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建加密器
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// 创建写入器（小文件大小以便触发滚动）
	writer, err := NewWALWriterWithConfig(tempDir, encryptor, 500, 5*time.Minute, 5)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// 写入多条记录
	for i := 0; i < 10; i++ {
		data := []byte("Test record " + string(rune('0'+i)))
		err := writer.Write(data)
		if err != nil {
			t.Fatalf("Failed to write record %d: %v", i, err)
		}
	}

	// 关闭写入器确保文件被刷新
	writer.Close()

	// 验证文件数量
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	t.Logf("Files in temp dir: %d", len(files))
	for _, f := range files {
		t.Logf("  - %s", f.Name())
	}

	if len(files) == 0 {
		t.Error("Should have at least one WAL file")
	}

	// 验证文件命名规范
	walFileCount := 0
	for _, file := range files {
		name := file.Name()
		if strings.HasSuffix(name, FileExtension) {
			walFileCount++
		}
	}

	if walFileCount == 0 {
		t.Error("Should have at least one .wal.enc file")
	}

	t.Logf("✅ WAL rolling test passed (created %d WAL files)", walFileCount)
}

// TestWALReader 测试读取解密
func TestWALReader(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "wal-reader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建加密器（使用固定密钥）
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// 创建写入器
	writer, err := NewWALWriter(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// 写入测试数据
	testRecords := []string{
		"First record",
		"Second record",
		"Third record",
	}

	for _, record := range testRecords {
		err := writer.Write([]byte(record))
		if err != nil {
			t.Fatalf("Failed to write record: %v", err)
		}
	}

	writer.Close()

	// 使用相同的加密器创建读取器
	reader, err := NewWALReader(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// 读取所有数据
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read records: %v", err)
	}

	// 验证读取的记录数量
	if len(records) != len(testRecords) {
		t.Errorf("Expected %d records, got %d", len(testRecords), len(records))
	}

	// 验证每条记录的内容
	for i, expected := range testRecords {
		if i < len(records) {
			if string(records[i]) != expected {
				t.Errorf("Record %d mismatch: got %s, want %s", i, string(records[i]), expected)
			}
		}
	}

	t.Log("✅ WAL reader test passed")
}

// TestWALRecovery 测试恢复
func TestWALRecovery(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "wal-recovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建加密器和写入器
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	writer, err := NewWALWriter(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// 写入一些数据
	testData := []byte("Critical data for recovery test")
	err = writer.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	writer.Close()

	// 使用相同的加密器创建读取器（模拟从 K8s Secret 恢复密钥）
	reader, err := NewWALReader(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// 读取恢复的数据
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to recover data: %v", err)
	}

	// 验证恢复的数据
	if len(records) == 0 {
		t.Fatal("No records recovered")
	}

	if string(records[0]) != string(testData) {
		t.Errorf("Recovered data mismatch: got %s, want %s", string(records[0]), string(testData))
	}

	t.Log("✅ WAL recovery test passed")
}

// TestK8sSecretLoader 测试 K8s Secret 加载器（可选）
func TestK8sSecretLoader(t *testing.T) {
	// 创建临时目录模拟 K8s Secret
	tempDir, err := os.MkdirTemp("", "k8s-secret-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 写入测试密钥
	testKey := []byte("0123456789abcdef0123456789abcdef") // 32 字节
	err = os.WriteFile(filepath.Join(tempDir, "key"), testKey, 0644)
	if err != nil {
		t.Fatalf("Failed to write key file: %v", err)
	}

	// 写入版本号
	err = os.WriteFile(filepath.Join(tempDir, "version"), []byte("42"), 0644)
	if err != nil {
		t.Fatalf("Failed to write version file: %v", err)
	}

	// 创建加载器
	loader := &K8sSecretLoader{secretPath: tempDir}

	// 加载密钥
	key, version, err := loader.LoadKey()
	if err != nil {
		t.Fatalf("Failed to load key: %v", err)
	}

	// 验证密钥
	if len(key) != 32 {
		t.Errorf("Key should be 32 bytes, got %d", len(key))
	}

	if version != 42 {
		t.Errorf("Version should be 42, got %d", version)
	}

	t.Log("✅ K8s Secret loader test passed")
}
