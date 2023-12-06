package mpd

func NewMPD() MPD {
	return &MPDtype{
		XMLNsAttr: "urn:mpeg:dash:schema:mpd:2011",
		ProfilesAttr: "urn:mpeg:dash:profile:isoff-live:2011",
		TypeAttr: "dynamic",
	}
}