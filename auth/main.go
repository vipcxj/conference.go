package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
)

const (
	AUTH_USAGE = "conference"
)

type AuthInfo struct {
	Usage     string   `json:"usage"`
	Key       string   `json:"key"`
	UID       string   `json:"uid"`
	UName     string   `json:"uname"`
	Role      string   `json:"role"`
	Rooms     []string `json:"rooms"`
	AutoJoin  bool     `json:"autoJoin"`
	Timestamp int64    `json:"timestamp"`
	Deadline  uint64   `json:"deadline"`
	Nonce     int32    `json:"nonce"`
}

func NewAuthInfoFromForm(form url.Values) (*AuthInfo, error) {
	key := form.Get("key")
	if key == "" {
		return nil, errors.InvalidParam("The param key is required for creating auth info.")
	}
	uid := form.Get("uid")
	if uid == "" {
		return nil, errors.InvalidParam("The param uid is required for creating auth info.")
	}
	uname := form.Get("uname")
	if uname == "" {
		return nil, errors.InvalidParam("The param uname is required for creating auth info.")
	}
	role := form.Get("role")
	if role == "" {
		return nil, errors.InvalidParam("The param role is required for creating auth info.")
	}
	var rooms []string
	rooms, ok := form["room"]
	if !ok || rooms == nil {
		return nil, errors.InvalidParam("The param room is required for creating auth info.")
	}
	var autoJoin bool
	autoJoinStr := form.Get("autojoin")
	autoJoinStr = strings.ToLower(autoJoinStr)
	if autoJoinStr == "1" || autoJoinStr == "on" || autoJoinStr == "true" || autoJoinStr == "yes" {
		autoJoin = true
	}
	var deadline uint64
	var err error
	deadlineStr := form.Get("deadline")
	if deadlineStr == "" {
		deadline = 5 * 60 * 1000
	} else {
		deadline, err = strconv.ParseUint(deadlineStr, 10, 0)
		if err != nil {
			return nil, errors.InvalidParam("The param deadline must be an unsigned integer.")
		}
	}
	strNonce := form.Get("nonce")
	if strNonce == "" {
		return nil, errors.InvalidParam("The param nonce is required for creating auth info.")
	}
	nonce, err := strconv.Atoi(strNonce)
	if err != nil {
		return nil, err
	}
	return &AuthInfo{
		Usage:     AUTH_USAGE,
		Key:       key,
		UID:       uid,
		UName:     uname,
		Role:      role,
		Rooms:     rooms,
		AutoJoin:  autoJoin,
		Timestamp: time.Now().Unix(),
		Deadline:  deadline,
		Nonce:     int32(nonce),
	}, nil
}

func _sha1(data []byte) ([]byte, error) {
	h := sha1.New()
	_, err := h.Write(data)
	if err != nil {
		return nil, nil
	}
	return h.Sum(nil), nil
}

func GenAesKey(pass string) ([]byte, error) {
	data := []byte(pass)
	hashs, err := _sha1(data)
	if err != nil {
		return nil, err
	}
	hashs, err = _sha1(hashs)
	if err != nil {
		return nil, err
	}
	return hashs[0:16], nil
}

func encrypt(plaintext []byte, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.FatalError("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func Encode(authInfo *AuthInfo) (string, error) {
	bytes, err := json.Marshal(authInfo)
	if err != nil {
		return "", err
	}
	key, err := GenAesKey(config.Conf().SecretKey)
	if err != nil {
		return "", err
	}
	bytes, err = encrypt(bytes, key)
	if err != nil {
		return "", err
	}
	encrypted := base64.StdEncoding.EncodeToString(bytes)
	return encrypted, err
}

func Decode(token string, authInfo *AuthInfo) error {
	bytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return err
	}
	key, err := GenAesKey(config.Conf().SecretKey)
	if err != nil {
		return err
	}
	bytes, err = decrypt(bytes, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, authInfo)
}
