package signal

import (
	"net/http"
	"strconv"

	"github.com/valyala/fasttemplate"
)

type ConferenceCallback struct {
	url string
}

func NewConferenceCallback(template string, ctx *SignalContext) *ConferenceCallback {
	authInfo := ctx.AuthInfo
	url := ""
	if template != "" {
		urlTemplate := fasttemplate.New(template, "{{", "}}")
		url = urlTemplate.ExecuteString(map[string]interface{}{
			"key":           authInfo.Key,
			"uid":           authInfo.UID,
			"uname":         authInfo.UName,
			"nonce":         strconv.FormatInt(int64(authInfo.Nonce), 10),
			"closeCallback": CloseCallback(ctx.Id),
		})
	}
	return &ConferenceCallback{
		url: url,
	}
}

func (me *ConferenceCallback) Call(ctx *SignalContext) (int, error) {
	if me.url == "" {
		return 0, nil
	}
	resp, err := http.Get(me.url)
	if err != nil {
		ctx.Sugar().Warnf("onclose callback invoked failed, %v", err)
		return 0, err
	}
	return resp.StatusCode, nil
}
