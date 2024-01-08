package common_test

import (
	"testing"

	"github.com/vipcxj/conference.go/pkg/common"
	"github.com/vipcxj/conference.go/utils"
)

func TestGetFieldByPath(t *testing.T) {
	obj := map[string]interface{}{
		"a": struct {
			B int
		}{1},
	}
	real, err := common.GetFieldByPath("a.B", obj)
	if err != nil {
		t.Error(err)
	}
	utils.AssertEqual(t, 1, real)
}
