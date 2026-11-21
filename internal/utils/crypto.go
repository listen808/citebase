package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/utils/config"
	"github.com/listen808/citebase/internal/utils/envname"
)

// devKeyOnce 保证开发回退密钥只生成一次，避免并发调用 getCryptoKey 时读写共享变量。
var devKeyOnce sync.Once

// getCryptoKey 获取 32 字节 AES-256 密钥。
// 优先使用配置 Security.EncryptKey，其次环境变量 AIWIKI_ENCRYPT_KEY；
// 两者皆空时回退到「本机首次运行生成并持久化」的随机密钥（data/.encrypt_key）。
// 旧实现使用可预测的源码内字面量，任何能读到仓库的人都能解密已入库的数据源凭据。
func getCryptoKey() []byte {
	key := config.AppConfig.Security.EncryptKey
	if key == "" {
		key = envname.Getenv("CITEBASE_ENCRYPT_KEY", "AIWIKI_ENCRYPT_KEY")
	}
	if key == "" {
		devKeyOnce.Do(warnDevKey)
		key = devFallbackKey()
	}
	sum := sha256.Sum256([]byte(key))
	return sum[:]
}

func warnDevKey() {
	msg := "Security.EncryptKey / CITEBASE_ENCRYPT_KEY 未配置（改名前的旧名同样接受），已启用本机随机密钥 data/.encrypt_key；此前用内置默认密钥加密的数据源口令需重新录入"
	if envname.Getenv("CITEBASE_ENV", "AIWIKI_ENV") == "production" {
		logx.Panic(msg)
	}
	logx.Warn(msg)
}

// devFallbackKey 读取或首次生成本机随机加密密钥（与 .jwt_secret 同一套做法）。
// 生成失败时退回一次性随机内存密钥，绝不回退到固定字面量。
func devFallbackKey() string {
	dataDir := envname.Getenv("CITEBASE_DATA_DIR", "AIWIKI_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	keyFile := filepath.Join(dataDir, ".encrypt_key")
	if data, err := os.ReadFile(keyFile); err == nil && len(data) >= 32 {
		return string(data)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand 不可用时不落地稳定密钥，避免留下可预测的持久密钥
		logx.Warnf("生成加密密钥失败，本次进程使用临时密钥: %v", err)
		return base64.StdEncoding.EncodeToString(buf)
	}
	key := base64.StdEncoding.EncodeToString(buf)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		logx.Warnf("创建数据目录失败，本次进程使用临时加密密钥: %v", err)
		return key
	}
	if err := os.WriteFile(keyFile, []byte(key), 0600); err != nil {
		logx.Warnf("写入 .encrypt_key 失败，本次进程使用临时加密密钥: %v", err)
	}
	return key
}

// Encrypt 使用 AES-GCM 加密明文，返回 base64(nonce || ciphertext)。
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(getCryptoKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密 Encrypt 生成的 base64 密文，返回明文。
func Decrypt(b64 string) (string, error) {
	if b64 == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(getCryptoKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:ns], data[ns:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptSecret 写入前加密。空串原样返回（Encrypt 自己也是这个约定，
// 免得调用方为"没填口令"这一档各写一个 if）。
func EncryptSecret(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return Encrypt(plaintext)
}

// DecryptSecretOrPlain 读出后解密；解不出来就原样返回。
//
// 这一条"退回原文"是为了兼容历史数据：口令字段在本次修复之前是明文入库的
// （数据源与邮件 SMTP 两处），改完只保证**新写入**的是密文，老库里的行仍然是明文。
// 判据用的是 AES-GCM 会不会验签通过，而不是"像不像 base64"——
// 真·base64 的明文口令会被 gcm.Open 拒掉，因而不会被误判成密文；
// 而密文一定能解开。等旧行被重新保存一次之后就自动变成密文，无需迁移脚本。
//
// 代价要说清楚：换了加密密钥（没配 Security.EncryptKey 时每次装机会重新生成
// data/.encrypt_key）之后，密文解不开会被当成"明文"返回给 DSN，
// 表现是连接失败而不是崩溃，运维要重新录入口令 —— 这与改动前的行为一致，
// 且 getCryptoKey 已经在换密钥时打过 warning。
func DecryptSecretOrPlain(stored string) string {
	if stored == "" {
		return ""
	}
	plain, err := Decrypt(stored)
	if err != nil {
		return stored
	}
	return plain
}

// LooksEncrypted 判断一个已存储的值是不是本函数产出的密文。
// 用于"提交回来的口令要不要覆盖旧值"：密文回填说明前端只是把列表里的值原样
// 传回来，不该把它当新口令再加密一遍（否则会在密文上再套一层）。
func LooksEncrypted(stored string) bool {
	if stored == "" {
		return false
	}
	_, err := Decrypt(stored)
	return err == nil
}
