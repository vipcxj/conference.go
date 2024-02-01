// Package samplebuilder builds media frames from RTP packets.
package samplebuilder

import (
	"fmt"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4/pkg/media"
)

type packet struct {
	start, end bool
	packet     *rtp.Packet
	buff       *[]byte
}

// SampleBuilder buffers packets and produces media frames
type SampleBuilder struct {
	// a circular array of buffered packets.  We interpret head=tail
	// as an empty builder, so there's always a free slot.  The
	// invariants that this data structure obeys are codified in the
	// function check below.
	packets    []packet
	// o x x x x x x o
	//   t           h
	head, tail uint16

	maxLate              uint16
	depacketizer         rtp.Depacketizer
	packetReleaseHandler func(*rtp.Packet, *[]byte)
	sampleRate           uint32

	// indicates whether the lastSeqno field is valid
	lastSeqnoValid bool
	// the seqno of the last popped or dropped packet, if any
	lastSeqno uint16

	// indicates whether the lastTimestamp field is valid
	lastTimestampValid bool
	// the timestamp of the last popped packet, if any.
	lastTimestamp uint32
}

// New constructs a new SampleBuilder.
//
// maxLate is the maximum delay, in RTP sequence numbers, that the samplebuilder
// will wait before dropping a frame.  The actual buffer size is twice as large
// in order to compensate for delays between Push and Pop.
func New(maxLate uint16, depacketizer rtp.Depacketizer, sampleRate uint32, opts ...Option) *SampleBuilder {
	if maxLate < 2 {
		maxLate = 2
	}
	if maxLate > 0x7FFF {
		maxLate = 0x7FFF
	}
	s := &SampleBuilder{
		packets:      make([]packet, 2*maxLate+1),
		maxLate:      maxLate,
		depacketizer: depacketizer,
		sampleRate:   sampleRate,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// An Option configures a SampleBuilder
type Option func(o *SampleBuilder)

// WithPacketReleaseHandler sets a callback that is called when the
// builder is about to release some packet.
func WithPacketReleaseHandler(h func(*rtp.Packet, *[]byte)) Option {
	return func(s *SampleBuilder) {
		s.packetReleaseHandler = h
	}
}

// check verifies the samplebuilder's invariants.  It may be used in testing.
func (s *SampleBuilder) check() {
	if s.head == s.tail {
		return
	}

	// the entry at tail must not be missing
	if s.packets[s.tail].packet == nil {
		panic("tail is missing")
	}
	// the entry at head-1 must not be missing
	if s.packets[s.dec(s.head)].packet == nil {
		panic("head is missing")
	}
	if s.lastSeqnoValid {
		// the last dropped packet is before tail
		diff := s.packets[s.tail].packet.SequenceNumber - s.lastSeqno
		if diff == 0 || diff&0x8000 != 0 { // diff <= 0 -> tailSeqno <= s.lastSeqno
			panic("lastSeqno is after tail")
		}
	}

	// indices are sequential, and the start and end flags are correct
	tailSeqno := s.packets[s.tail].packet.SequenceNumber
	for i := uint16(0); i < uint16(s.Len()); i++ {
		index := (s.tail + i) % uint16(len(s.packets))
		if s.packets[index].packet == nil {
			continue
		}
		if s.packets[index].packet.SequenceNumber != tailSeqno+i {
			panic("wrong seqno")
		}
		ts := s.packets[index].packet.Timestamp
		if index != s.tail && !s.packets[index].start {
			prev := s.dec(index)
			if s.packets[prev].packet != nil && s.packets[prev].packet.Timestamp != ts {
				panic("start is not set")
			}
		}
		if index != s.dec(s.head) && !s.packets[index].end {
			next := s.inc(index)
			if s.packets[next].packet != nil && s.packets[next].packet.Timestamp != ts {
				panic("end is not set")
			}
		}
	}
	// all packets outside of the interval are missing
	for i := s.head; i != s.tail; i = s.inc(i) {
		if s.packets[i].packet != nil {
			panic("packet is set")
		}
	}
}

// Len returns the difference minus one between the smallest and the
// largest sequence number stored in the SampleBuilder.
func (s *SampleBuilder) Len() int {
	if s.tail <= s.head {
		return int(s.head - s.tail)
	}
	return int(s.head + uint16(len(s.packets)) - s.tail)
}

// cap returns the capacity of the SampleBuilder.
func (s *SampleBuilder) cap() uint16 {
	// since head==tail indicates an empty builder, we always keep one
	// empty element
	return uint16(len(s.packets)) - 1
}

// inc adds one to an index.
func (s *SampleBuilder) inc(n uint16) uint16 {
	if n < uint16(len(s.packets))-1 {
		return n + 1
	}
	return 0
}

// dec subtracts one from an index.
func (s *SampleBuilder) dec(n uint16) uint16 {
	if n > 0 {
		return n - 1
	}
	return uint16(len(s.packets)) - 1
}

// isStart invokes the PartitionHeadChecker associted with s.
func (s *SampleBuilder) isStart(p *rtp.Packet) bool {
	return s.depacketizer.IsPartitionHead(p.Payload)
}

// isEnd invokes the partitionTailChecker associated with s.
func (s *SampleBuilder) isEnd(p *rtp.Packet) bool {
	return s.depacketizer.IsPartitionTail(p.Marker, p.Payload)
}

func (s *SampleBuilder) oldestPacket() *rtp.Packet {
	return s.packets[s.tail].packet
}

func (s *SampleBuilder) oldestBuff() *[]byte {
	return s.packets[s.tail].buff
}

// release releases the last packet.
// clean packets[s.tail] and s.tail to s.tail + x where packets[s.tail + x] is not empty or packets is empty
func (s *SampleBuilder) release() bool {
	if s.head == s.tail {
		return false
	}
	// o x o x x o
	//   t       h
	s.lastSeqnoValid = true
	s.lastSeqno = s.oldestPacket().SequenceNumber
	if s.packetReleaseHandler != nil {
		s.packetReleaseHandler(s.oldestPacket(), s.oldestBuff())
	}
	// o o o x x o
	//     t     h   
	s.packets[s.tail] = packet{}
	s.tail = s.inc(s.tail)
	// o o o x x o
	//       t   h 
	for s.tail != s.head && s.packets[s.tail].packet == nil {
		s.tail = s.inc(s.tail)
	}
	if s.tail == s.head {
		s.head = 0
		s.tail = 0
	}
	return true
}

// releaseAll releases all packets.
func (s *SampleBuilder) releaseAll() {
	for s.tail != s.head {
		s.release()
	}
}

// drop drops the last frame, even if it is incomplete.  It returns true
// if a packet has been dropped, and the dropped packet's timestamp.
func (s *SampleBuilder) drop() (bool, uint32) {
	if s.tail == s.head {
		return false, 0
	}
	// o x o x x x x o
	//   t           h
	ts := s.oldestPacket().Timestamp
	s.release()
	// o o o x x x x o
	//       t       h
	for s.tail != s.head {
		if s.packets[s.tail].start ||
			s.oldestPacket().Timestamp != ts {
			break
		}
		s.release()
	}
	if !s.lastTimestampValid {
		s.lastTimestamp = ts
		s.lastTimestampValid = true
	}
	return true, ts
}

// Push adds an RTP Packet to s's buffer.
//
// Push does not copy the input: the packet will be retained by s.  If you
// plan to reuse the packet or its buffer, make sure to perform a copy.
func (s *SampleBuilder) Push(p *rtp.Packet, buf *[]byte) {
	if s.lastSeqnoValid {
		if (s.lastSeqno-p.SequenceNumber) & 0x8000 == 0 { // s.lastSeqno >= p.SequenceNumber
			// late packet
			if s.lastSeqno - p.SequenceNumber > s.maxLate {
				s.lastSeqnoValid = false
			} else {
				return
			}
		} else {
			last := p.SequenceNumber - s.maxLate
			if (last - s.lastSeqno) & 0x8000 == 0 { // last >= s.lastSeqno
				if s.head != s.tail { // not empty
					seqno := s.oldestPacket().SequenceNumber - 1
					if (last - seqno) & 0x8000 == 0 { // last >= seqno
						last = seqno
					}
				}
				s.lastSeqno = last
			}
		}
	}

	if s.head == s.tail {
		// empty
		s.packets[0] = packet{
			start:  s.isStart(p),
			end:    s.isEnd(p),
			packet: p,
			buff:   buf,
		}
		s.tail = 0
		s.head = 1
		return
	}

	seqno := p.SequenceNumber
	ts := p.Timestamp
	newest := s.dec(s.head)
	newestSeqno := s.packets[newest].packet.SequenceNumber
	if seqno == newestSeqno {
		// duplicate packet
		if s.packetReleaseHandler != nil {
			s.packetReleaseHandler(p, buf)
		}
		return
	}
	if seqno == newestSeqno + 1 {
		// sequential
		if s.tail == s.inc(s.head) { // full
			dropped, droppedTs := s.drop()
			// if part of frame has been dropped
			if dropped && droppedTs == ts {
				if s.tail != s.head {
					panic(fmt.Errorf("this is impossible"))
				}
				if s.packetReleaseHandler != nil {
					s.packetReleaseHandler(p, buf)
				}
				return
			}
		}
		// put p at head and inc head

		start := false
		// drop may have dropped the whole buffer
		if s.tail != s.head {
			start = s.packets[newest].end ||
				s.packets[newest].packet.Timestamp != p.Timestamp ||
				s.isStart(p)
			if start {
				s.packets[newest].end = true
			}
		} else {
			start = s.isStart(p)
		}
		s.packets[s.head] = packet{
			start:  start,
			end:    s.isEnd(p),
			packet: p,
			buff:   buf,
		}
		s.head = s.inc(s.head)
		return
	}

	if ((seqno - newestSeqno) & 0x8000) == 0 { // seqno >= newestSeqno -> seqno - newestSeqno >= 2
		// packet in the future
		offset := seqno - newestSeqno - 1
		count := offset + 1
		if count > s.cap() {
			s.releaseAll()
			s.Push(p, buf)
			return
		}
		// make free space
		for uint16(s.Len()) + count > s.cap() {
			dropped, _ := s.drop()
			if !dropped {
				// this shouldn't happen
				if s.packetReleaseHandler != nil {
					s.packetReleaseHandler(p, buf)
				}
				return
			}
		}
		// x o x o x x x o
		//   h t          
		// o x o o x x x o
		//   t           h
		if s.tail == s.head {
			s.packets[0] = packet{
				start:  s.isStart(p),
				end:    s.isEnd(p),
				packet: p,
				buff:   buf,
			}
			s.tail = 0
			s.head = 1
		} else {
			index := (s.head + offset) % uint16(len(s.packets))
			start := s.isStart(p)
			s.packets[index] = packet{
				start:  start,
				end:    s.isEnd(p),
				packet: p,
				buff:   buf,
			}
			s.head = s.inc(index)
		}
		return
	}

	// packet is in the past
	offset := newestSeqno - seqno + 1
	if offset >= s.cap() {
		// too old
		if s.packetReleaseHandler != nil {
			s.packetReleaseHandler(p, buf)
		}
		return
	}
	// o x o o x x x o x o o
	//     h   t         
	// o o x x o x x o x o o
	//     t             h
	var index uint16
	if s.head >= offset {
		index = s.head - offset
	} else {
		index = s.head + uint16(len(s.packets)) - offset
	}

	// extend if necessary
	if s.tail < s.head {
		// buffer is contigous
		if index < s.tail || index > s.head {
			s.tail = index
		}
	} else {
		// buffer is discontigous
		if index < s.tail && index > s.head {
			s.tail = index
		}
	}

	if s.packets[index].packet != nil {
		// duplicate packet
		if s.packetReleaseHandler != nil {
			s.packetReleaseHandler(p, buf)
		}
		return
	}

	// compute start and end flags, both for us and our neighbours
	start := s.isStart(p)
	// if index == s.tail, then s.tail is reset because of p is older than last tail
	if index != s.tail {
		prev := s.dec(index)
		if s.packets[prev].packet != nil {
			if s.packets[prev].packet.Timestamp != ts {
				start = true
			}
			if !start {
				start = s.packets[prev].end
			} else {
				s.packets[prev].end = true
			}
		}
	}
	end := s.isEnd(p)
	next := s.inc(index)
	if s.packets[next].packet != nil {
		if s.packets[next].packet.Timestamp != ts {
			end = true
		}
		if !end {
			end = s.packets[next].start
		} else {
			s.packets[next].start = true
		}
	}

	// done!
	s.packets[index] = packet{
		start:  start,
		end:    end,
		packet: p,
		buff:   buf,
	}
}

func (s *SampleBuilder) pop(force bool) *media.Sample {
again:
	if s.tail == s.head {
		return nil
	}

	if !s.packets[s.tail].start {
		diff := s.packets[s.dec(s.head)].packet.SequenceNumber -
			s.oldestPacket().SequenceNumber
		if force || diff > s.maxLate {
			s.drop()
			goto again
		}
		return nil
	}

	seqno := s.oldestPacket().SequenceNumber
	if !force && s.lastSeqnoValid && s.lastSeqno+1 != seqno {
		// packet loss before tail, so wait the loss packet
		return nil
	}

	ts := s.oldestPacket().Timestamp
	last := s.tail
	// find end
	for last != s.head && !s.packets[last].end {
		if s.packets[last].packet == nil {
			if force {
				s.drop()
				goto again
			}
			return nil
		}
		last = s.inc(last)
	}

	if last == s.head {
		return nil
	}

	var data []byte
	count := last - s.tail + 1
	if last < s.tail {
		count = s.cap() + last - s.tail + 1
	}
	for i := uint16(0); i < count; i++ {
		buf, err := s.depacketizer.Unmarshal(
			s.packets[s.tail].packet.Payload,
		)
		s.release()
		if err != nil {
			return nil
		}
		data = append(data, buf...)
	}

	var samples uint32
	if s.lastTimestampValid {
		samples = ts - s.lastTimestamp
	}

	s.lastTimestampValid = true
	s.lastTimestamp = ts
	duration := time.Duration(float64(samples) / float64(s.sampleRate) * float64(time.Second))

	return &media.Sample{
		Data:            data,
		PacketTimestamp: ts,
		Duration:        duration,
	}
}

// Pop returns a completed packet.  If the oldest packet is incomplete and
// hasn't reached MaxLate yet, Pop returns nil.
func (s *SampleBuilder) Pop() *media.Sample {
	return s.pop(false)
}

// ForcePopWithTimestamp is like PopWithTimestamp, but will always pops
// a sample if any are available, even if it's being blocked by a missing
// packet.  This is useful when the stream ends, or after a link outage.
// After ForcePopWithTimestamp returns nil, the samplebuilder is
// guaranteed to be empty.
func (s *SampleBuilder) ForcePop() *media.Sample {
	return s.pop(true)
}

func (s *SampleBuilder) Close() {
	s.releaseAll()
}