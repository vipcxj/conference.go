package signal

import (
	"net/http"
	"strconv"

	"github.com/valyala/fasttemplate"
)

type ConferenceCallback struct {
	name string
	url string
}

func NewConferenceCallback(name string, template string, ctx *SignalContext) *ConferenceCallback {
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
		name: name,
		url: url,
	}
}

func (me *ConferenceCallback) Call(ctx *SignalContext) (int, error) {
	if me.url == "" {
		ctx.Sugar().Warnf("invalid callback %s, no url specified.", me.name)
		return 0, nil
	}
	resp, err := http.Get(me.url)
	if err != nil {
		ctx.Sugar().Warnf("callback %s invoked failed, %v", me.name, err)
		return 0, err
	} else {
		ctx.Sugar().Debugf("callback %s return status code %d", me.name, resp.StatusCode)
	}
	return resp.StatusCode, nil
}
