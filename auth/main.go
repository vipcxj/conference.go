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

	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
)

type AuthInfo struct {
	UID string `json:"uid"`
	UName string `json:"uname"`
	Role string `json:"role"`
	Room string `json:"room"`
	Nonce int32 `json:"nonce"`
}

func NewAuthInfoFromForm(form url.Values) (*AuthInfo, error) {
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
	room := form.Get("room")
	if room == "" {
		return nil, errors.InvalidParam("The param room is required for creating auth info.")
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
		UID: uid,
		UName: uname,
		Role: role,
		Room: room,
		Nonce: int32(nonce),
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

func genAesKey(pass string) ([]byte, error) {
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
	key, err := genAesKey(config.Conf().SecretKey)
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
	key, err := genAesKey(config.Conf().SecretKey)
	if err != nil {
		return err
	}
	bytes, err = decrypt(bytes, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, authInfo)
}