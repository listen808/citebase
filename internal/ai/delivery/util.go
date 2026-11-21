package delivery

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// fmtInt 整数转字符串。
func fmtInt(i int) string {
	return strconv.Itoa(i)
}

// hmacHex 用 HMAC-SHA256 对消息签名并返回十六进制字符串。
func hmacHex(secret string, msg []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(msg)
	return hex.EncodeToString(mac.Sum(nil))
}
