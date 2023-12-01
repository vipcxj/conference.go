package segmenter

import (
	"github.com/Eyevinn/mp4ff/mp4"
)

type Segmenter struct {

}

func (s *Segmenter) Init() {
	mp4.NewFile()
}