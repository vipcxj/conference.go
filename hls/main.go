package hls

import (
	"github.com/pion/rtp"
)

type HLS struct {
}

func (h *HLS) WriteRTP(packet rtp.Packet) {
	// https://github.com/yapingcat/gomedia/blob/main/example/example_hls_fmp4_h265.go#L79
}
