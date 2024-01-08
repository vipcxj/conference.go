package auth_test

import (
	"testing"

	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/utils"
)

func TestGenKey(t *testing.T) {
	key := "123456"
	out, err := auth.GenAesKey(key)
	if err != nil {
		t.Error(err)
		return
	}
	expected := []byte{107, 180, 131, 126, 183, 67, 41, 16, 94, 228, 86, 141, 218, 125, 198, 126}
	utils.AssertEqualSlice(t, expected, out)
}
