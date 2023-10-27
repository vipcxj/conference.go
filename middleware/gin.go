package middleware

import (
	"net/http"

	"github.com/vipcxj/conference.go/errors"

	"github.com/gin-gonic/gin"
)

func Cors(cors string) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", cors) // 可将将 * 替换为指定的域名
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
			c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}
		c.Next()
	}
}

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
