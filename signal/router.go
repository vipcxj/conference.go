package signal

import (
	"fmt"
	"net"
	"sync"

	_ "github.com/pion/webrtc/v4"
	"github.com/zishang520/engine.io/types"
	_ "github.com/zishang520/socket.io/v2/socket"
)

type Packet struct {

}

type Router struct {
	server net.PacketConn
	ser_mu sync.Mutex
	remove_mu sync.Mutex
	chans  map[string]chan *Packet
	transports  map[string]*types.Set[string]
}

func (r *Router) makeSureServer(ip string, port int16) (net.PacketConn, error) {
	if r.server != nil {
		return r.server, nil
	}
	r.ser_mu.Lock()
	if r.server != nil {
		r.ser_mu.Unlock()
		return r.server, nil
	}
	defer r.ser_mu.Unlock()
	ser, err := net.ListenPacket("udp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}
	r.server = ser
	return r.server, nil
}

func (r *Router) AddTrack(trackId string, transportId string) chan *Packet {
	r.remove_mu.Lock()
	defer r.remove_mu.Unlock()
	_, ok := r.transports[transportId]
	if !ok {
		r.transports[transportId] = &types.Set[string]{}
	}
	c, ok := r.chans[trackId]
	if !ok {
		c = make(chan *Packet, 128)
		r.chans[trackId] = c
	}
	return c
}

func (r *Router) RemoveTrack(trackId string) {
	r.remove_mu.Lock()
	defer r.remove_mu.Unlock()
	c, ok := r.chans[trackId]
	if !ok {
		return
	}
	delete(r.chans, trackId)
	close(c)
	for key, transport := range r.transports {
		transport.Delete(trackId)
		if transport.Len() == 0 {
			delete(r.transports, key)
		}
	}
}