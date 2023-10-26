package middleware

import (
	"github.com/vipcxj/conference.go/errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		if myErr, ok := err.(*errors.ConferenceError); ok {
			var code int
			if myErr.Code > 600 {
				code = http.StatusInternalServerError
			} else {
				code = myErr.Code
			}
			c.JSON(code, gin.H{
				"code": myErr.Code,
				"msg":  myErr.Msg,
				"data": myErr.Data,
			})
		} else {
			var msg string
			if myErr, ok := err.(error); ok {
				msg = myErr.Error()
			} else if myErr, ok := err.(string); ok {
				msg = myErr
			} else {
				msg = ""
			}
			// 若非自定义错误则返回详细错误信息err.Error()
			// 比如save session出错时设置的err
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": http.StatusInternalServerError,
				"msg":  msg,
				"data": nil,
			})
		}
	})
}