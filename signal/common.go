package signal

type Track struct {
	PubId    string `json:"pubId" mapstructure:"pubId"`
	GlobalId string `json:"globalId" mapstructure:"globalId"`
	// only vaild in local
	LocalId string `json:"localId" mapstructure:"localId"`
	// used to bind local and remote track
	BindId   string            `json:"bindId" mapstructure:"bindId"`
	StreamId string            `json:"streamId" mapstructure:"streamId"`
	Labels   map[string]string `json:"labels" mapstructure:"labels"`
}

func (me *Track) MatchLabel(name, value string) bool {
	v, ok := me.Labels[name]
	if ok {
		return v == value
	} else {
		return false
	}
}

func (me *Track) HasLabel(name string) bool {
	if me.Labels == nil {
		return false
	}
	_, ok := me.Labels[name]
	return ok
}
