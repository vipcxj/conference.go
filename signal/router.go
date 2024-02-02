package signal

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/google/uuid"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
)

const MAGIC_SETUP = "consetup"
const MAGIC_ARK = "conneark"
const MAGIC_PACKET = "subr"
const MAGIC_EOF = "eofp"

type SetupPacket struct {
	// should be MAGIC_SETUP
	Magic       [8]byte
	TransportId [16]byte
}

func (p *SetupPacket) GetTransportId() (id uuid.UUID) {
	id, err := uuid.FromBytes(p.TransportId[:])
	if err != nil {
		panic(err)
	}
	return id
}

func NewSetupPacket(id *uuid.UUID) *SetupPacket {
	packet := SetupPacket{}
	copy(packet.Magic[:], []byte(MAGIC_SETUP))
	if id != nil {
		copy(packet.TransportId[:], id[:])
	}
	return &packet
}

func NewSetupPacketBuf(id *uuid.UUID) []byte {
	out := make([]byte, 24)
	copy(out, []byte(MAGIC_SETUP))
	copy(out[8:], id[:])
	return out
}

func IsSetup(data []byte) bool {
	if len(data) < 24 {
		return false
	}
	return bytes.Equal(data[0:8], []byte(MAGIC_SETUP))
}

func DecodeSetupPacket(buf []byte, n int, packet *SetupPacket) error {
	if n != 24 {
		return errors.BadPacket(fmt.Sprintf("Invalid connect setup packet, wrong size: %d, should be 24", n))
	}
	if string(buf[0:8]) == MAGIC_SETUP {
		packet.TransportId = [16]byte(buf[8:24])
		return nil
	} else {
		return errors.BadPacket(fmt.Sprintf("Invalid connect setup packet, wrong magic: %s, should be \"%s\"", string(buf[0:8]), MAGIC_SETUP))
	}
}

type SetupArkPacket struct {
	// should be MAGIC_ARK
	Magic       [8]byte
	TransportId [16]byte
}

func (p *SetupArkPacket) GetTransportId() (id uuid.UUID) {
	id, err := uuid.FromBytes(p.TransportId[:])
	if err != nil {
		panic(err)
	}
	return id
}

func NewSetupArkPacket(id *uuid.UUID) *SetupArkPacket {
	packet := SetupArkPacket{}
	copy(packet.Magic[:], []byte(MAGIC_ARK))
	if id != nil {
		copy(packet.TransportId[:], id[:])
	}
	return &packet
}

func NewSetupArkPacketBuf(id *uuid.UUID) []byte {
	out := make([]byte, 24)
	copy(out, []byte(MAGIC_ARK))
	copy(out[8:], id[:])
	return out
}

func IsSetupArk(data []byte) bool {
	if len(data) < 24 {
		return false
	}
	return bytes.Equal(data[0:8], []byte(MAGIC_ARK))
}

func IsSetupId(data []byte, id uuid.UUID) bool {
	if len(data) < 24 {
		return false
	}
	return bytes.Equal(data[8:24], id[:])
}

func (p *SetupArkPacket) Marshal() []byte {
	out := make([]byte, 24)
	copy(out, p.Magic[:])
	copy(out[8:], p.TransportId[:])
	return out
}

func DecodeSetupArkPacket(buf []byte, n int, packet *SetupArkPacket) error {
	if n != 24 {
		return errors.BadPacket(fmt.Sprintf("Invalid connect setup ark packet, wrong size: %d, should be 24", n))
	}
	if string(buf[0:8]) == MAGIC_ARK {
		packet.TransportId = [16]byte(buf[8:24])
		return nil
	} else {
		return errors.BadPacket(fmt.Sprintf("Invalid connect setup ark packet, wrong magic: %s, should be \"%s\"", string(buf[0:8]), MAGIC_ARK))
	}
}

const PACKET_MAGIC_SIZE = 4
const PACKET_TRACK_HEADER_SIZE = PACKET_MAGIC_SIZE + 16
const PACKET_MAX_SIZE = 1600

type Packet struct {
	TrackId string
	data    [PACKET_MAX_SIZE]byte
	n       int
}

func (p *Packet) IsEOF() bool {
	return IsEofPacket(p.data[:p.n])
}

func (p *Packet) IsData() bool {
	return IsDataPacket(p.data[:p.n])
}

var packetPool = sync.Pool{
	New: func() interface{} {
		return &Packet{}
	},
}

func resetPacketPoolAllocation(packet *Packet) {
	*packet = Packet{}
	packetPool.Put(packet)
}

func getPacketAllocationFromPool() *Packet {
	p := packetPool.Get()
	return p.(*Packet)
}

func NewEOFPacket(trackId string) (*Packet, error) {
	id, err := uuid.Parse(trackId)
	if err != nil {
		return nil, err
	}
	p := getPacketAllocationFromPool()
	p.TrackId = trackId
	copy(p.data[0:PACKET_MAGIC_SIZE], []byte(MAGIC_EOF))
	copy(p.data[PACKET_MAGIC_SIZE:PACKET_TRACK_HEADER_SIZE], id[:])
	p.n = PACKET_TRACK_HEADER_SIZE
	return p, nil
}

func NewDataPacket(trackId string, raw *rtp.Packet) (*Packet, error) {
	id, err := uuid.Parse(trackId)
	if err != nil {
		return nil, err
	}
	p := getPacketAllocationFromPool()
	p.TrackId = trackId
	n := raw.MarshalSize()
	if n > PACKET_MAX_SIZE-PACKET_TRACK_HEADER_SIZE {
		resetPacketPoolAllocation(p)
		return nil, errors.FatalError(fmt.Sprintf("Too big rtp packet, size: %d, max support size %d", n, PACKET_MAX_SIZE-PACKET_TRACK_HEADER_SIZE))
	}
	n, err = raw.MarshalTo(p.data[PACKET_TRACK_HEADER_SIZE:])
	if err != nil {
		resetPacketPoolAllocation(p)
		return nil, err
	}
	p.n = n + PACKET_TRACK_HEADER_SIZE
	copy(p.data[0:PACKET_MAGIC_SIZE], []byte(MAGIC_PACKET))
	copy(p.data[PACKET_MAGIC_SIZE:PACKET_TRACK_HEADER_SIZE], id[:])
	return p, nil
}

func NewPacketFromBuf(trackId string, buf []byte) (*Packet, error) {
	id, err := uuid.Parse(trackId)
	if err != nil {
		return nil, err
	}
	p := getPacketAllocationFromPool()
	p.TrackId = trackId
	p.n = len(buf) + PACKET_TRACK_HEADER_SIZE
	copy(p.data[0:PACKET_MAGIC_SIZE], []byte(MAGIC_PACKET))
	copy(p.data[PACKET_MAGIC_SIZE:PACKET_TRACK_HEADER_SIZE], id[:])
	copy(p.data[PACKET_TRACK_HEADER_SIZE:], buf)
	return p, nil
}

func IsDataPacket(data []byte) bool {
	if len(data) <= 20 {
		return false
	}
	return bytes.Equal(data[0:4], []byte(MAGIC_PACKET))
}

func IsEofPacket(data []byte) bool {
	if len(data) != PACKET_TRACK_HEADER_SIZE {
		return false
	}
	return bytes.Equal(data[0:PACKET_MAGIC_SIZE], []byte(MAGIC_EOF))
}

func ExtractPacket(data []byte) (trackId string, rtpData []byte, err error) {
	id, err := uuid.FromBytes(data[PACKET_MAGIC_SIZE:PACKET_TRACK_HEADER_SIZE])
	if err != nil {
		return
	}
	trackId = id.String()
	if len(data) > PACKET_TRACK_HEADER_SIZE {
		rtpData = data[PACKET_TRACK_HEADER_SIZE:]
	}
	return
}

const CHAN_COUNT = 4
const CHAN_BUF_SIZE = 512
const DEFAULT_MTU = 1500

type Accepter struct {
	track  *webrtc.TrackLocalStaticRTP
	closed bool
	subs   map[string]*Subscription
	mu     sync.Mutex
}

func NewAccepter(track *Track) *Accepter {
	var capability webrtc.RTPCodecCapability
	if track.Codec != nil {
		capability = RTPCodecParametersToWebrtc(track.Codec).RTPCodecCapability
	} else {
		capability = webrtc.RTPCodecCapability{
			MimeType: webrtc.MimeTypeVP8,
		}
	}
	newId := NewUUID(track.LocalId, track.PubId)
	newSId := NewUUID(track.StreamId, track.PubId)
	trackLocal, err := webrtc.NewTrackLocalStaticRTP(capability, newId, newSId)
	if err != nil {
		panic(err)
	}
	return &Accepter{
		track: trackLocal,
	}
}

func (a *Accepter) BindSub(sub *Subscription) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.subs == nil {
		a.subs = map[string]*Subscription{}
	}
	_, ok := a.subs[sub.id]
	if !ok {
		a.subs[sub.id] = sub
	}
}

func (a *Accepter) Write(buf []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return io.ErrClosedPipe
	}
	for _, sub := range a.subs {
		sub.ctx.Metrics().OnWebrtcRtpWrite(sub.ctx, len(buf))
	}
	_, err := a.track.Write(buf)
	return err
}

func (a *Accepter) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	for _, sub := range a.subs {
		sub.UnbindAccepter(a)
	}
	a.subs = nil
}

type Router struct {
	global        *Global
	server        *net.UDPConn
	id            uuid.UUID
	mu            sync.Mutex
	incomingChans [CHAN_COUNT]chan *Packet
	curChan       int
	// track id -> transport id set
	track2transports *haxmap.Map[string, *haxmap.Map[string, int]]
	track2accepters  *haxmap.Map[string, *Accepter]
	// transport id -> udp addr
	addrMap *haxmap.Map[string, string]
	// addr -> conn
	externalTransports map[string]*net.UDPConn
}

var ROUTER *Router = &Router{}

func NewRouter(global *Global) (*Router, error) {
	router := &Router{
		global: global,
		track2transports: haxmap.New[string, *haxmap.Map[string, int]](),
		track2accepters: haxmap.New[string, *Accepter](),
		addrMap: haxmap.New[string, string](),
		externalTransports: map[string]*net.UDPConn{},
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, errors.FatalError("unable to create router, %v", err)
	}
	router.id = id
	for i := 0; i < CHAN_COUNT; i++ {
		router.incomingChans[i] = make(chan *Packet, CHAN_BUF_SIZE)
	}
	return router, nil
}

func (r *Router) Conf() *config.ConferenceConfigure {
	return r.global.Conf()
}

func (r *Router) Addr() string {
	r.makeSureServer()
	return r.Conf().RouterExternalAddress()
}

func (r *Router) makeSureServer() (*net.UDPConn, error) {
	if r.server != nil {
		return r.server, nil
	}
	r.mu.Lock()
	if r.server != nil {
		r.mu.Unlock()
		return r.server, nil
	}
	defer r.mu.Unlock()

	addr, err := net.ResolveUDPAddr("udp", r.Conf().RouterListenAddress())
	if err != nil {
		return nil, err
	}
	ser, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	log.Sugar().Infof("start server on ", ser.LocalAddr())
	r.server = ser
	go r.run(ser)
	return r.server, nil
}

func (r *Router) run(ser *net.UDPConn) {
	buf := make([]byte, DEFAULT_MTU)
	go func() {
		for {
			n, addr, err := ser.ReadFromUDP(buf)
			if err != nil {
				log.Sugar().Errorf("error happened when read from udp: ", err)
				continue
			}
			setup := NewSetupPacket(nil)
			err = DecodeSetupPacket(buf, n, setup)
			if err != nil {
				log.Sugar().Errorln(err)
				continue
			}
			connId, _ := uuid.FromBytes(setup.TransportId[:])
			connIdStr := connId.String()
			r.addrMap.Set(connIdStr, addr.String())
			ark := NewSetupArkPacketBuf((*uuid.UUID)(&setup.TransportId))
			ser.WriteToUDP(ark, addr)
		}
	}()
	go func() {
		for {
			func() {
				defer errors.Ignore(io.ErrClosedPipe, webrtc.ErrConnectionClosed)
				var packet *Packet
				select {
				case packet = <-r.incomingChans[0]:
				case packet = <-r.incomingChans[1]:
				case packet = <-r.incomingChans[2]:
				case packet = <-r.incomingChans[3]:
				}
				defer resetPacketPoolAllocation(packet)
				transports, ok := r.track2transports.Get(packet.TrackId)
				if ok {
					transports.ForEach(func(transport string, v int) bool {
						addrStr, ok := r.addrMap.Get(transport)
						if ok {
							addr, err := net.ResolveUDPAddr("udp", addrStr)
							if err != nil {
								panic(err)
							}
							ser.WriteToUDP(packet.data[:packet.n], addr)
						}
						return true
					})
				}
				accepter, ok := r.track2accepters.Get(packet.TrackId)
				if ok {
					if packet.IsEOF() {
						r.CloseAccepter(packet.TrackId)
					} else {
						err := accepter.Write(packet.data[PACKET_TRACK_HEADER_SIZE:packet.n])
						if err != nil {
							panic(err)
						}
					}
				}
			}()
		}
	}()
}

func (r *Router) CloseAccepter(trackId string) {
	accepter, ok := r.track2accepters.GetAndDel(trackId)
	if ok {
		accepter.Close()
	}
}

func (r *Router) PublishTrack(trackId string, transportId string) chan *Packet {
	r.makeSureServer()
	r.mu.Lock()
	defer r.mu.Unlock()
	if transportId != r.id.String() {
		ts, ok := r.track2transports.Get(trackId)
		if !ok {
			ts = haxmap.New[string, int]()
			r.track2transports.Set(trackId, ts)
		}
		ts.Set(transportId, 0)
	}
	c := r.incomingChans[r.curChan]
	r.curChan = (r.curChan + 1) % CHAN_COUNT
	return c
}

func (r *Router) RemoveTrack(trackId string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.track2transports.Del(trackId)
}

func (r *Router) makeSureExternelConn(addr string) (conn *net.UDPConn) {
	if r.Addr() == addr {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	conn, ok := r.externalTransports[addr]
	if ok {
		return
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		panic(err)
	}
	conn, err = net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		panic(err)
	}
	log.Sugar().Infof("set up external udp connect from %v to %v", r.Addr(), addr)
	chArk := make(chan bool)
	chClose := make(chan interface{})
	go func() {
		for {
			select {
			case <-chArk:
				return
			case <-chClose:
				return
			default:
				conn.Write(NewSetupPacketBuf(&r.id))
				time.Sleep(time.Millisecond * 50)
			}
		}
	}()
	go func() {
		buf := make([]byte, PACKET_MAX_SIZE)
		closer := func() {
			close(chClose)
			r.mu.Lock()
			defer r.mu.Unlock()
			oldConn, ok := r.externalTransports[addr]
			if ok && oldConn == conn {
				delete(r.externalTransports, addr)
			}
			// ignore close error, who care.
			conn.Close()
		}
		for {
			n, err := conn.Read(buf)
			if err == io.EOF {
				closer()
				return
			}
			if err != nil {
				panic(err)
			}
			s := buf[0:n]
			if IsDataPacket(s) {
				trackId, rtpData, err := ExtractPacket(s)
				if err != nil {
					panic(err)
				}
				accepter, ok := r.track2accepters.Get(trackId)
				if ok {
					err = accepter.Write(rtpData)
					if err != nil {
						panic(err)
					}
				}
			} else if IsSetupArk(s) {
				if IsSetup(s) {
					chArk <- true
				}
			} else if IsEofPacket(s) {
				trackId, _, err := ExtractPacket(s)
				if err != nil {
					panic(err)
				}
				_, ok := r.track2accepters.Get(trackId)
				if ok {
					r.CloseAccepter(trackId)
				}
			}
		}
	}()
	r.externalTransports[addr] = conn
	return
}

func (r *Router) AcceptTrack(track *Track) *Accepter {
	accepter, _ := r.track2accepters.GetOrCompute(track.GlobalId, func() *Accepter {
		return NewAccepter(track)
	})
	return accepter
}
