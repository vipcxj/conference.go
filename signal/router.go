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
	return IsEofPacket(p.data[:])
}

func (p *Packet) IsData() bool {
	return IsDataPacket(p.data[:])
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

type SubscribationWant struct {
	ctx *SingalContext
}

type Subscribation struct {
	track *webrtc.TrackLocalStaticRTP
	closed bool
	closers []func()
	mu sync.Mutex
}

func NewSubscribation() *Subscribation {
	track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{
		MimeType: webrtc.MimeTypeVP8,
	}, uuid.NewString(), uuid.NewString())
	if err != nil {
		panic(err)
	}
	return &Subscribation{
		track: track,
		closers: []func(){},
	}
}

func (sub *Subscribation) Write(to []byte) (int, error) {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if sub.closed {
		return 0, io.ErrClosedPipe
	}
	return sub.track.Write(to)
}

func (sub *Subscribation) Close() {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	sub.closed = true
	for _, closer := range sub.closers {
		closer()
	}
}

type Router struct {
	ip            string
	server        *net.UDPConn
	id            uuid.UUID
	mu            sync.Mutex
	incomingChans [CHAN_COUNT]chan *Packet
	curChan       int
	// track id -> transport id set
	track2transports *haxmap.Map[string, *haxmap.Map[string, int]]
	track2subscribations     *haxmap.Map[string, *Subscribation]
	pendingSubscribations     map[string]map[SubscribationWant]bool
	// transport id -> udp addr
	addrMap *haxmap.Map[string, string]
	// addr -> conn
	externalTransports map[string]*net.UDPConn
}

var ROUTER *Router = &Router{}

func InitRouter() error {
	ROUTER.ip = config.Conf().Ip
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	ROUTER.id = id
	for i := 0; i < CHAN_COUNT; i++ {
		ROUTER.incomingChans[i] = make(chan *Packet, CHAN_BUF_SIZE)
	}
	ROUTER.track2transports = haxmap.New[string, *haxmap.Map[string, int]]()
	ROUTER.track2subscribations = haxmap.New[string, *Subscribation]()
	ROUTER.pendingSubscribations = map[string]map[SubscribationWant]bool{}
	ROUTER.addrMap = haxmap.New[string, string]()
	ROUTER.externalTransports = map[string]*net.UDPConn{}
	return nil
}

func GetRouter() *Router {
	return ROUTER
}

func (r *Router) Addr() string {
	ser := r.server
	if ser == nil {
		return ""
	}
	addr, _ := ser.LocalAddr().(*net.UDPAddr)
	port := addr.Port
	return fmt.Sprintf("%s:%d", r.ip, port)
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
	ser, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(config.Conf().Ip), Port: config.Conf().Port})
	if err != nil {
		return nil, err
	}
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
				fmt.Println("error happened when read from udp: ", err)
				continue
			}
			setup := NewSetupPacket(nil)
			err = DecodeSetupPacket(buf, n, setup)
			if err != nil {
				fmt.Println(err.Error())
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
							addr, err := net.ResolveUDPAddr("", addrStr)
							if err != nil {
								panic(err)
							}
							ser.WriteToUDP(packet.data[:packet.n], addr)
						}
						return true
					})
				}
				sub, ok := r.track2subscribations.Get(packet.TrackId)
				if ok {
					if packet.IsEOF() {
						r.CloseSubscribation(packet.TrackId)
					} else {
						_, err := sub.Write(packet.data[PACKET_TRACK_HEADER_SIZE:packet.n])
						if err != nil {
							panic(err)
						}
					}
				}
			}()
		}
	}()
}

func (r *Router) CloseSubscribation(trackId string) {
	sub, ok := r.track2subscribations.GetAndDel(trackId)
	if ok {
		sub.Close()
	}
}

func (r *Router) PublishTrack(trackId string, transportId string) chan *Packet {
	r.makeSureServer()
	r.mu.Lock()
	defer r.mu.Unlock()
	ts, ok := r.track2transports.Get(trackId)
	if !ok {
		ts = haxmap.New[string, int]()
		r.track2transports.Set(trackId, ts)
	}
	ts.Set(transportId, 0)
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

	chArk := make(chan bool)
	chClose := make(chan bool)
	go func() {
		for {
			select {
			case <- chArk:
				return
			case <- chClose:
				return
			default:
				conn.Write(NewSetupPacketBuf(&r.id))
				time.Sleep(time.Millisecond * 50)
			}
		}
	}()
	go func() {
		buf := make([]byte, PACKET_MAX_SIZE)
		for {
			n, err := conn.Read(buf)
			if err == io.EOF {
				chClose <- true
				r.mu.Lock()
				defer r.mu.Unlock()
				oldConn, ok := r.externalTransports[addr]
				if ok && oldConn == conn {
					delete(r.externalTransports, addr)
				}
				// ignore close error, who care.
				conn.Close()
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
				sub, ok := r.track2subscribations.Get(trackId)
				if ok {
					sub.Write(rtpData)
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
				_, ok := r.track2subscribations.Get(trackId)
				if ok {
					r.CloseSubscribation(trackId)
				}
			}
		}
	}()
	r.externalTransports[addr] = conn
	return
}

func (r *Router) WantTracks(tracks []*Track, ctx *SingalContext) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, track := range tracks {
		peers, ok := r.pendingSubscribations[track.GlobalId]
		if !ok {
			peers = map[SubscribationWant]bool{}
			r.pendingSubscribations[track.GlobalId] = peers
		}
		peers[SubscribationWant{ ctx: ctx }] = true
	}
}

func (r *Router) SubscribeTrackIfWanted(globalId string, trackId string, streamId string, addr string) (track *webrtc.TrackLocalStaticRTP) {
	r.mu.Lock()
	defer r.mu.Unlock()
	wants, ok := r.pendingSubscribations[globalId]
	if !ok {
		return
	}
	if len(wants) == 0 {
		return
	}
	r.makeSureServer()
	if addr != "" {
		r.makeSureExternelConn(addr)
	}
	sub, _ := r.track2subscribations.GetOrCompute(globalId, func() *Subscribation {
		return NewSubscribation()
	})
	for want, _ := range wants {
		want.ctx.Subscribed(sub, globalId)
	}
	delete(r.pendingSubscribations, globalId)
	return
}
