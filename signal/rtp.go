package signal

import (
	"fmt"
	"sync"

	"github.com/pion/rtp"
	"go.uber.org/zap"
)

type rtpStats struct {
	pkgs       []*rtp.Header
	head       int16
	tail       int16
	drops      []*rtp.Header
	drops_head int16
	drops_tail int16
	all        int32
	lost       int32
	logger     *zap.Logger
	sugar      *zap.SugaredLogger
	mu         sync.Mutex
}

func NewRtpStats(cap int, logger *zap.Logger) *rtpStats {
	return &rtpStats{
		pkgs:   make([]*rtp.Header, cap),
		drops:  make([]*rtp.Header, cap),
		logger: logger,
		sugar:  logger.Sugar(),
	}
}

func (s *rtpStats) capacity() int16 {
	return int16(len(s.pkgs))
}

func (s *rtpStats) dropsCapacity() int16 {
	return int16(len(s.drops))
}

func (s *rtpStats) stepHead() {
	s.head = (s.head + 1) % s.capacity()
}

func (s *rtpStats) backHead(offset int16) {
	s.head = (s.head - offset) % s.capacity()
}

func (s *rtpStats) stepTail() {
	s.tail = (s.tail + 1) % s.capacity()
}

func (s *rtpStats) forwardTail(offset int16) {
	s.tail = (s.tail + offset) % s.capacity()
}

func (s *rtpStats) size() int16 {
	if s.tail > s.head {
		return s.tail - s.head
	} else {
		return s.tail + s.capacity() - s.head
	}
}

func (s *rtpStats) dorpsSize() int16 {
	if s.drops_tail > s.drops_head {
		return s.drops_tail - s.drops_head
	} else {
		return s.drops_tail + s.dropsCapacity() - s.drops_head
	}
}

func (s *rtpStats) full() bool {
	return s.size() == s.capacity()-1
}

func (s *rtpStats) empty() bool {
	return s.head == s.tail
}

func (s *rtpStats) dropsFull() bool {
	return s.dorpsSize() == s.dropsCapacity()-1
}

func (s *rtpStats) hasSpace(space int16) bool {
	return s.capacity()-1-s.size() >= space
}

func (s *rtpStats) oldestPkg() *rtp.Header {
	if !s.empty() {
		return s.pkgs[s.head]
	} else {
		return nil
	}
}

func (s *rtpStats) latestPkg() *rtp.Header {
	if !s.empty() {
		i := s.tail - 1
		if i < 0 {
			i += s.capacity()
		}
		return s.pkgs[i]
	} else {
		return nil
	}
}

func (s *rtpStats) drop() {
	if !s.empty() {
		pkg := s.pkgs[s.head]
		s.pkgs[s.head] = nil
		s.stepHead()
		if s.dropsFull() {
			s.all++
			if pkg == nil {
				s.lost++
			}
			s.drops[s.drops_head] = pkg
			s.drops_head = (s.drops_head + 1) % s.dropsCapacity()
			s.drops_tail = (s.drops_tail + 1) % s.dropsCapacity()
		} else {
			s.drops[s.drops_tail] = pkg
			s.drops_tail = (s.drops_tail + 1) % s.dropsCapacity()
		}
	}
}

func (s *rtpStats) Push(buf []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	header := rtp.Header{}
	_, err := header.Unmarshal(buf)
	if err != nil {
		return fmt.Errorf("unable to unmarshal the rtp buf, %w", err)
	}
	if s.empty() {
		s.pkgs[s.head] = &header
		s.stepTail()
		return nil
	}
	if header.SequenceNumber < s.oldestPkg().SequenceNumber {
		for {
			offset := s.oldestPkg().SequenceNumber - header.SequenceNumber
			if !s.hasSpace(int16(offset)) {
				s.drop()
			} else {
				s.backHead(int16(offset))
				s.pkgs[s.head] = &header
				return nil
			}
		}
	} else if header.SequenceNumber > s.latestPkg().SequenceNumber {
		for {
			offset := header.SequenceNumber - s.latestPkg().SequenceNumber
			if !s.hasSpace(int16(offset)) {
				s.drop()
			} else {
				s.forwardTail(int16(offset))
				var i int
				if s.tail >= 1 {
					i = int(s.tail) - 1
				} else {
					i = int(s.tail) + int(s.capacity()) - 1
				}
				s.pkgs[i] = &header
				return nil
			}
		}
	} else {
		offset := header.SequenceNumber - s.oldestPkg().SequenceNumber
		i := (s.head + int16(offset)) % s.capacity()
		if s.pkgs[i] == nil {
			s.pkgs[i] = &header
			return nil
		}
	}
	return nil
}

func (s *rtpStats) Print() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sugar.Debugf("all pkg: %d, lost pkg: %d", s.all, s.lost)
}
