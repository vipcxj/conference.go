package signal

import (
	"fmt"
	"net"
	"sync"

	"github.com/google/uuid"
	_ "github.com/pion/webrtc/v4"
	"github.com/vipcxj/conference.go/errors"
	"github.com/zishang520/engine.io/types"
	_ "github.com/zishang520/socket.io/v2/socket"
)

const MAGIC_SETUP = "consetup"

type SetupPacket struct {
	// should be 'consetup'
	Magic  [8]byte
	ConnId [16]byte
}

func NewSetupPacket() *SetupPacket {
	packet := SetupPacket{}
	copy(packet.Magic[:], []byte(MAGIC_SETUP))
	return &packet
}

func DecodeSetupPacket(buf []byte, n int, packet *SetupPacket) error {
	if n != 24 {
		return errors.BadPacket(fmt.Sprintf("Invalid connect setup packet, wrong size: %d, should be 24", n))
	}
	if string(buf[0:8]) == MAGIC_SETUP {
		packet.ConnId = [16]byte(buf[8:24])
		return nil
	} else {
		return errors.BadPacket(fmt.Sprintf("Invalid connect setup packet, wrong magic: %s, should be \"%s\"", string(buf[0:8]), MAGIC_SETUP))
	}
}

type Packet struct {
	TrackId string
	data    []byte
}

const CHAN_COUNT = 4
const CHAN_BUF_SIZE = 512
const DEFAULT_MTU = 1500

type Router struct {
	server        *net.UDPConn
	mu            sync.Mutex
	incomingChans [CHAN_COUNT]chan *Packet
	curChan       int
	// track id -> connection id set
	track2transports types.Map[string, []string]
	// connection id -> udp addr
	addrMap types.Map[string, string]
}

func (r *Router) makeSureServer(ip string, port int) (*net.UDPConn, error) {
	if r.server != nil {
		return r.server, nil
	}
	r.mu.Lock()
	if r.server != nil {
		r.mu.Unlock()
		return r.server, nil
	}
	defer r.mu.Unlock()
	ser, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(ip), Port: port})
	if err != nil {
		return nil, err
	}
	go r.run(ser)
	r.server = ser
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
			setup := NewSetupPacket()
			err = DecodeSetupPacket(buf, n, setup)
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			connId, _ := uuid.FromBytes(setup.ConnId[:])
			connIdStr := connId.String()
			r.addrMap.Store(connIdStr, addr.String())
		}
	}()
	go func() {
		var packet *Packet
		select {
		case packet = <-r.incomingChans[0]:
		case packet = <-r.incomingChans[1]:
		case packet = <-r.incomingChans[2]:
		case packet = <-r.incomingChans[3]:
		}
		transports, ok := r.track2transports.Load(packet.TrackId)
		if ok {
			for _, transport := range transports {
				addrStr, ok := r.addrMap.Load(transport)
				if ok {
					addr, err := net.ResolveUDPAddr("", addrStr)
					if err != nil {
						panic(err)
					}
					ser.WriteToUDP(packet.data, addr)
				}
			}
		}
	}()
}

func (r *Router) PublishTrack(trackId string, transportId string) chan *Packet {
	r.makeSureServer("0.0.0.0", 0)
	r.mu.Lock()
	defer r.mu.Unlock()
	ts, ok := r.track2transports.Load(trackId)
	if !ok {
		ts = []string{transportId}
	} else {
		ts = append(ts, transportId)
	}
	r.track2transports.Store(trackId, ts)
	c := r.incomingChans[r.curChan]
	if c == nil {
		c = make(chan *Packet, CHAN_BUF_SIZE)
		r.incomingChans[r.curChan] = c
	}
	r.curChan = (r.curChan + 1) % CHAN_COUNT
	return c
}

func (r *Router) RemoveTrack(trackId string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.track2transports.Delete(trackId)
}
