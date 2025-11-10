package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// EncryptionManager 加密管理器
// 仅使用 AES-256-GCM 进行数据库加密
type EncryptionManager struct {
	masterKey []byte // 用于数据库加密的 256-bit 主密钥
	mu        sync.RWMutex
}

var (
	instance *EncryptionManager
	once     sync.Once
)

// GetEncryptionManager 获取加密管理器实例
func GetEncryptionManager() (*EncryptionManager, error) {
	var initErr error
	once.Do(func() {
		instance, initErr = newEncryptionManager()
	})
	return instance, initErr
}

// newEncryptionManager 初始化加密管理器
func newEncryptionManager() (*EncryptionManager, error) {
	em := &EncryptionManager{}

	// 加载或生成数据库主密钥
	if err := em.loadOrGenerateMasterKey(); err != nil {
		return nil, fmt.Errorf("初始化主密钥失败: %w", err)
	}

	log.Println("🔐 加密管理器初始化成功 (AES-256-GCM)")
	return em, nil
}

// ==================== 主密钥管理 ====================

const (
	masterKeyFile = "crypto/.secrets/master.key"
	keySize       = 32 // 256-bit key for AES-256
)

// loadOrGenerateMasterKey 加载或生成主密钥
func (em *EncryptionManager) loadOrGenerateMasterKey() error {
	// 确保目录存在
	secretsDir := filepath.Dir(masterKeyFile)
	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return err
	}

	// 尝试加载现有密钥
	if _, err := os.Stat(masterKeyFile); err == nil {
		return em.loadMasterKey()
	}

	// 生成新密钥
	log.Println("🔑 生成新的 AES-256 主密钥...")
	masterKey := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, masterKey); err != nil {
		return fmt.Errorf("生成主密钥失败: %w", err)
	}

	// 保存密钥到文件
	if err := os.WriteFile(masterKeyFile, masterKey, 0600); err != nil {
		return fmt.Errorf("保存主密钥失败: %w", err)
	}

	em.masterKey = masterKey
	log.Println("✅ 主密钥已生成并保存")
	return nil
}

// loadMasterKey 从文件加载主密钥
func (em *EncryptionManager) loadMasterKey() error {
	keyData, err := os.ReadFile(masterKeyFile)
	if err != nil {
		return fmt.Errorf("读取主密钥文件失败: %w", err)
	}

	if len(keyData) != keySize {
		return fmt.Errorf("主密钥长度无效: 期望 %d 字节，实际 %d 字节", keySize, len(keyData))
	}

	em.masterKey = keyData
	log.Println("✅ 主密钥已加载")
	return nil
}

// ==================== AES-256-GCM 加密/解密 ====================

// EncryptForDatabase 使用主密钥加密数据（用于数据库存储）
func (em *EncryptionManager) EncryptForDatabase(plaintext string) (string, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if len(em.masterKey) == 0 {
		return "", errors.New("主密钥未初始化")
	}

	// 创建 AES 密码块
	block, err := aes.NewCipher(em.masterKey)
	if err != nil {
		return "", fmt.Errorf("创建 AES 密码块失败: %w", err)
	}

	// 创建 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 模式失败: %w", err)
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// 返回 base64 编码的结果
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptFromDatabase 使用主密钥解密数据（从数据库读取）
func (em *EncryptionManager) DecryptFromDatabase(encryptedBase64 string) (string, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	// 处理空字符串（未加密的旧数据）
	if encryptedBase64 == "" {
		return "", nil
	}

	if len(em.masterKey) == 0 {
		return "", errors.New("主密钥未初始化")
	}

	// base64 解码
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}

	// 创建 AES 密码块
	block, err := aes.NewCipher(em.masterKey)
	if err != nil {
		return "", fmt.Errorf("创建 AES 密码块失败: %w", err)
	}

	// 创建 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 模式失败: %w", err)
	}

	// 检查数据长度
	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("密文长度不足")
	}

	// 提取 nonce 和密文
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败: %w", err)
	}

	return string(plaintext), nil
}

// ==================== 向后兼容的方法 ====================

// EncryptDatabaseData 加密数据库数据（向后兼容）
func (em *EncryptionManager) EncryptDatabaseData(plaintext string) (string, error) {
	return em.EncryptForDatabase(plaintext)
}

// DecryptDatabaseData 解密数据库数据（向后兼容）
func (em *EncryptionManager) DecryptDatabaseData(encryptedBase64 string) (string, error) {
	return em.DecryptFromDatabase(encryptedBase64)
}

// GetPublicKeyPEM 获取公钥（已弃用，返回空字符串）
func (em *EncryptionManager) GetPublicKeyPEM() string {
	log.Println("⚠️  GetPublicKeyPEM 方法已弃用，RSA 加密已移除")
	return ""
}

// DecryptWithPrivateKey 使用私钥解密数据（已弃用，使用 AES 解密）
func (em *EncryptionManager) DecryptWithPrivateKey(encryptedBase64 string) (string, error) {
	log.Println("⚠️  DecryptWithPrivateKey 方法已弃用，使用 AES-256-GCM 解密")
	return em.DecryptFromDatabase(encryptedBase64)
}

// RotateMasterKey 轮换主密钥（简化版）
func (em *EncryptionManager) RotateMasterKey() error {
	log.Println("⚠️  RotateMasterKey 方法已弃用，主密钥轮换功能已简化")
	log.Println("如需轮换密钥，请手动删除 crypto/.secrets/master.key 文件并重启服务")
	return nil
}

// GetMasterKeyInfo 获取主密钥信息（用于调试，不返回实际密钥）
func (em *EncryptionManager) GetMasterKeyInfo() string {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if len(em.masterKey) == 0 {
		return "主密钥未初始化"
	}

	return fmt.Sprintf("AES-256 主密钥已加载 (长度: %d 字节)", len(em.masterKey))
}
