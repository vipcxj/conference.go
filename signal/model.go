package signal

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/valyala/fasttemplate"
	"github.com/vipcxj/conference.go/auth"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/model"
	"github.com/vipcxj/conference.go/pkg/common"
	sgt "github.com/vipcxj/conference.go/pkg/segmenter"
	"github.com/vipcxj/conference.go/proto"
	"github.com/vipcxj/conference.go/utils"
	"go.uber.org/zap"
)

func IsRTPClosedError(err error) bool {
	return err == io.EOF || err == io.ErrClosedPipe || err.Error() == "interceptor is closed"
}

type SubscribedTrack struct {
	sub      *Subscription
	pubTrack *model.Track
	sid      string
	accepter *Accepter
	sender   *webrtc.RTPSender
	bindId   string
	labels   map[string]string
}

func (st *SubscribedTrack) Remove() {
	// ignore error
	st.sub.sctx.Peer.RemoveTrack(st.sender)
	delete(st.sub.acceptedTrack, st.pubTrack.GlobalId)
}

type Subscription struct {
	id            string
	mu            sync.Mutex
	closed        bool
	reqTypes      []string
	pattern       *model.PublicationPattern
	sctx          *SignalContext
	acceptedPubId string
	acceptedTrack map[string]*SubscribedTrack
}

func (s *Subscription) AcceptTrack(msg *model.StateMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.acceptedPubId != "" && s.acceptedPubId != msg.PubId {
		return
	}
	matcheds, unmatcheds := model.MatchTracks(s.pattern, msg.Tracks, s.reqTypes)
	if s.acceptedPubId == msg.PubId {
		for _, unmatch := range unmatcheds {
			subscribedTrack, ok := s.acceptedTrack[unmatch.GlobalId]
			if ok {
				subscribedTrack.Remove()
			}
		}
		if len(matcheds) == 0 {
			s.acceptedPubId = ""
			return
		}
	}
	if len(matcheds) == 0 {
		return
	}
	s.sctx.Sugar().Infof("accept pub %v", msg)
	s.acceptedPubId = msg.PubId
	router := s.sctx.Global.Router()
	// accept track from addr
	router.makeSureExternelConn(msg.Addr)
	peer, err := s.sctx.makeSurePeer()
	if err != nil {
		panic(err)
	}
	if s.acceptedTrack == nil {
		s.acceptedTrack = map[string]*SubscribedTrack{}
	}
	need_neg := false
	var subTracks []*model.Track
	for _, matched := range matcheds {
		track := model.CopyTrack(matched)
		subscribedTrack, ok := s.acceptedTrack[track.GlobalId]
		if !ok {
			accepter := router.AcceptTrack(track)
			sender, err := peer.AddTrack(accepter.track)
			if err != nil {
				panic(err)
			}
			need_neg = true
			// Read incoming RTCP packets
			// Before these packets are returned they are processed by interceptors. For things
			// like NACK this needs to be called.
			go func() {
				rtcpBuf := make([]byte, 1600)
				for {
					if _, _, err := sender.Read(rtcpBuf); err != nil {
						return
					}
				}
			}()
			mid := getMidFromSender(peer, sender)
			sid := accepter.track.StreamID()
			subscribedTrack = &SubscribedTrack{
				sub:      s,
				sid:      sid,
				pubTrack: track,
				bindId:   mid,
				accepter: accepter,
				sender:   sender,
				labels:   track.Labels,
			}
			s.acceptedTrack[track.GlobalId] = subscribedTrack
			accepter.BindSub(s)
			track.LocalId = accepter.track.ID()
			track.BindId = mid
			track.StreamId = sid
		} else {
			track.LocalId = subscribedTrack.accepter.track.ID()
			track.BindId = subscribedTrack.bindId
			track.StreamId = subscribedTrack.sid
		}
		subTracks = append(subTracks, track)
	}
	if need_neg {
		sdpId := s.sctx.NextSdpMsgId()
		s.sctx.Sugar().Infof("gen sdp id: %v", sdpId)

		s.sctx.clusterEmit(&model.SelectMessage{
			PubId:       msg.PubId,
			Tracks:      matcheds,
			TransportId: router.id.String(),
		})
		// s.ctx.Socket.To(s.ctx.Rooms()...).Emit("select", selMsg)
		// s.ctx.Socket.Emit("select", selMsg)
		respMsg := &model.SubscribedMessage{
			SubId:  s.id,
			PubId:  msg.PubId,
			SdpId:  sdpId,
			Tracks: subTracks,
		}
		go func() {
			err := s.sctx.MustEmitWithAck("subscribed", "send subscribed msg", respMsg)
			if err != nil {
				return
			}
			s.sctx.StartNegotiate(peer, sdpId)
		}()
	}
}

func (s *Subscription) UnbindAccepter(accepter *Accepter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// when closed, s.acceptedTrack is empty, so no need check closed
	for _, track := range s.acceptedTrack {
		if track.accepter == accepter {
			track.Remove()
		}
	}
}

func (s *Subscription) UpdatePattern(message *model.SubscribeMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// when closed, s.acceptedTrack is empty, so no need check closed
	tracks := utils.MapValuesTo(s.acceptedTrack, func(s string, st *SubscribedTrack) (mapped *model.Track, remove bool) {
		return st.pubTrack, false
	})
	_, unmatched := model.MatchTracks(message.Pattern, tracks, message.ReqTypes)
	for _, track := range unmatched {
		at, found := s.acceptedTrack[track.GlobalId]
		if found {
			at.Remove()
		}
	}
	s.reqTypes = message.ReqTypes
	s.pattern = message.Pattern
}

func (s *Subscription) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.sctx.subscriptions.Del(s.id)
	for _, track := range s.acceptedTrack {
		track.Remove()
	}
	s.acceptedTrack = nil
}

type Publication struct {
	id      string
	closeCh chan struct{}
	ctx     *SignalContext
	// after init not change, readonly, so no need mutex
	tracks      map[string]*PublishedTrack
	recordStart bool
	mux         sync.Mutex
}

func (me *Publication) Video() *PublishedTrack {
	_, pt, _ := utils.MapFindFirst(me.tracks, func(s string, pt *PublishedTrack) bool {
		rt := pt.remote
		if rt != nil && rt.Kind() == webrtc.RTPCodecTypeVideo {
			return true
		} else {
			return false
		}
	})
	return pt
}

func (me *Publication) Audio() *PublishedTrack {
	_, pt, _ := utils.MapFindFirst(me.tracks, func(s string, pt *PublishedTrack) bool {
		rt := pt.remote
		if rt != nil && rt.Kind() == webrtc.RTPCodecTypeAudio {
			return true
		} else {
			return false
		}
	})
	return pt
}

func (me *Publication) Conf() *config.ConferenceConfigure {
	return me.ctx.Global.Conf()
}

var RECORD_PACKET_POOL = &sync.Pool{
	New: func() any {
		buf := make([]byte, PACKET_MAX_SIZE)
		return &buf
	},
}

func BorrowRecordPacketBuf() *[]byte {
	return RECORD_PACKET_POOL.Get().(*[]byte)
}

func ReturnRecordPacketBuf(buf *[]byte) {
	RECORD_PACKET_POOL.Put(buf)
}

type PacketBox struct {
	pkg  *rtp.Packet
	buff *[]byte
	n    uint16
}

func extractLabelsFromLabeledTrack(t common.LabeledTrack) map[string]string {
	return t.Labels()
}

func (me *Publication) createRecordKey(tracks []common.LabeledTrack) string {
	keyTemplate := me.Conf().Record.DBIndex.Key
	if keyTemplate == "" {
		panic(fmt.Errorf("conf record.dbIndex.key should not be empty when dbIndex enabled"))
	}
	key := fasttemplate.ExecuteFuncString(keyTemplate, "{{", "}}", func(w io.Writer, tag string) (int, error) {
		tag = strings.TrimSpace(tag)
		tagL := strings.ToLower(tag)
		if strings.HasPrefix(tagL, "label:") {
			parts := strings.SplitN(tag, ":", 3)
			var labelName string
			if len(parts) > 1 {
				labelName = parts[1]
			} else {
				return 0, fmt.Errorf("label name is empty")
			}

			labelValue, ok := sgt.GetCommonLabel(labelName, tracks, extractLabelsFromLabeledTrack)
			if !ok {
				if len(parts) > 2 {
					labelValue = parts[2]
				} else {
					return 0, fmt.Errorf("invalid label %s", labelName)
				}
			}
			return fmt.Fprint(w, labelValue)
		} else {
			v, err := common.GetFieldByPath(tag, map[string]interface{}{
				"auth": me.ctx.AuthInfo,
			})
			if err != nil {
				return 0, fmt.Errorf("invalid tag in key template, %v", err)
			}
			return fmt.Fprintf(w, "%v", v)
		}
	})
	return key
}

func (me *Publication) makeSegmenter(tracks []common.LabeledTrack, sgtBox *segmenterBox) error {
	var recorder *Recorder
	if me.Conf().Record.DBIndex.Enable {
		recordKey := me.createRecordKey(tracks)
		if recordKey == "" {
			panic(fmt.Errorf("conf record.dbIndex.key should not be empty"))
		}
		recorder = NewRecorder(me.Conf(), recordKey, me.ctx.Global.Mongo())
	}
	dirTemplate := me.Conf().Record.DirPath
	indexTemplate := me.Conf().Record.IndexName
	if indexTemplate == "" {
		panic(fmt.Errorf("conf record.indexName should not be empty"))
	}
	segmentDuration := me.Conf().Record.SegmentDuration
	gopSize := me.Conf().Record.GopSize
	segmenter, err := sgt.NewSegmenter(
		tracks, dirTemplate, indexTemplate, segmentDuration, gopSize,
		sgt.WithBaseTemplate(me.Conf().Record.BasePath),
		sgt.WithPacketReleaseHandler(func(p *rtp.Packet, b *[]byte) {
			ReturnRecordPacketBuf(b)
		}),
		sgt.WithSegmentHandler(func(sc *sgt.SegmentContext) {
			if recorder != nil {
				go func() {
					_, err := recorder.Record(sc)
					if err != nil {
						me.ctx.Logger().Sugar().Errorf("record failed, %v", err)
					}
				}()
			}
		}),
		sgt.WithTemplateContext(map[string]interface{}{
			"auth": me.ctx.AuthInfo,
		}),
	)
	if err != nil {
		return err
	} else {
		sgtBox.segmenter = segmenter
		return nil
	}
}

type segmenterBox struct {
	segmenter *sgt.Segmenter
}

func NewSegmenterBox() *segmenterBox {
	return &segmenterBox{}
}

func (me *segmenterBox) Close() {
	if me != nil && me.segmenter != nil {
		me.segmenter.Close()
	}
}

func (me *Publication) startRecord() {
	enable := me.Conf().Record.Enable
	if !enable {
		return
	}

	me.mux.Lock()
	defer me.mux.Unlock()
	if me.recordStart {
		return
	}
	me.recordStart = true
	var tracks []common.LabeledTrack = utils.MapValuesTo(me.tracks, func(k string, v *PublishedTrack) (common.LabeledTrack, bool) {
		return v, false
	})

	sgtBox := NewSegmenterBox()
	err := me.makeSegmenter(tracks, sgtBox)
	if err != nil {
		panic(err)
	}
	pktCh := make(chan *PacketBox, 16)
	for _, track := range tracks {
		var onRTP RTPConsumer = func(ssrc webrtc.SSRC, data []byte, attrs interceptor.Attributes, err error) bool {
			if err != nil {
				pktCh <- &PacketBox{
					pkg: &rtp.Packet{
						Header: rtp.Header{
							SSRC: uint32(ssrc),
						},
					},
					buff: nil,
				}
				if !IsRTPClosedError(err) {
					panic(err)
				}
				return true
			}
			buf := BorrowRecordPacketBuf()
			n := copy(*buf, data)
			packet := &rtp.Packet{}
			err = packet.Unmarshal((*buf)[:len(data)])
			if err != nil {
				panic(err)
			}
			// fmt.Printf("receive packet with no %d at pts %d for track %d\n", packet.SequenceNumber, packet.Timestamp, packet.SSRC)
			pktCh <- &PacketBox{
				pkg:  packet,
				buff: buf,
				n:    uint16(n),
			}
			return false
		}
		track.(*PublishedTrack).OnRTPPacket(&onRTP)
	}
	go func() {
		defer sgtBox.Close()
		trackNum := len(tracks)
		closedTrackNum := 0
		var ready bool
		for {
			var pkg *PacketBox
			select {
			case <-me.closeCh:
				return
			case pkg = <-pktCh:
				if pkg.buff == nil {
					sgtBox.segmenter.CloseTrack(pkg.pkg.SSRC)
					closedTrackNum++
					continue
				}
			}
			if closedTrackNum == trackNum {
				return
			}
			ready, err = sgtBox.segmenter.TryReady()
			if err != nil {
				panic(err)
			}
			if !ready {
				continue
			}
			err := sgtBox.segmenter.WriteRtp(pkg.pkg, pkg.buff)
			if err != nil {
				if err == sgt.ErrBadDts {
					sgtBox.segmenter.Close()
					err = me.makeSegmenter(tracks, sgtBox)
					if err != nil {
						panic(err)
					}
					continue
				} else {
					panic(err)
				}
			}
		}
	}()
}

func (me *Publication) isAllBind() bool {
	return utils.MapAllMatch(me.tracks, func(s string, pt *PublishedTrack) bool {
		return pt.isBind()
	})
}

func (me *Publication) Bind() bool {
	for _, pt := range me.tracks {
		pt.Bind()
	}
	if me.isAllBind() {
		me.ctx.Sugar().Debugf("pub %s all bind", me.id)
		me.startRecord()
		r := me.ctx.Global.Router()
		tracks := utils.MapValuesTo(me.tracks, func(s string, pt *PublishedTrack) (mapped *proto.Track, remove bool) {
			return pt.track, false
		})
		me.ctx.clusterEmit(&model.StateMessage{
			PubId:  me.id,
			Tracks: tracks,
			Addr:   r.Addr(),
		})
		// me.ctx.Socket.To(me.ctx.Rooms()...).Emit("state", msg)
		// me.ctx.Socket.Emit("state", msg)
		return true
	} else {
		return false
	}
}

func (me *Publication) State(want *model.WantMessage) []*proto.Track {
	tracks := utils.MapValuesTo(me.tracks, func(s string, pt *PublishedTrack) (mapped *model.Track, remove bool) {
		return pt.track, !pt.isBind()
	})
	matched, _ := model.MatchTracks(want.Pattern, tracks, want.ReqTypes)
	return matched
}

func (me *Publication) SatifySelect(sel *model.SelectMessage) {
	for _, t := range sel.Tracks {
		pt, ok := me.tracks[t.GlobalId]
		if ok {
			pt.SatifySelect(sel.TransportId)
		}
	}
}

func (me *Publication) IsClosed() bool {
	select {
	case <-me.closeCh:
		return true
	default:
		return false
	}
}

func (me *Publication) Close() {
	me.ctx.publications.Del(me.id)
	close(me.closeCh)
}

type RTPConsumer func(ssrc webrtc.SSRC, data []byte, attrs interceptor.Attributes, err error) bool

type PublishedTrack struct {
	pub          *Publication
	mu           sync.Mutex
	track        *model.Track
	remote       *webrtc.TrackRemote
	consumers    []*RTPConsumer
	consumerMu   sync.Mutex
	consumerChan chan interface{}
}

func (me *PublishedTrack) Publication() *Publication {
	return me.pub
}

func (me *PublishedTrack) Track() *model.Track {
	return me.track
}

func (me *PublishedTrack) TrackRemote() *webrtc.TrackRemote {
	return me.remote
}

func (me *PublishedTrack) removeConsumer(consumer *RTPConsumer) {
	me.consumers = utils.SliceRemoveByValue(me.consumers, true, consumer)
	if len(me.consumers) == 0 {
		close(me.consumerChan)
		me.consumerChan = nil
	}
}

func (me *PublishedTrack) OnRTPPacket(consumer *RTPConsumer) (unon func()) {
	me.consumerMu.Lock()
	defer me.consumerMu.Unlock()
	me.consumers = append(me.consumers, consumer)
	unon = func() {
		me.consumerMu.Lock()
		defer me.consumerMu.Unlock()
		me.removeConsumer(consumer)
	}
	if len(me.consumers) == 1 {
		closeCh := make(chan interface{})
		me.consumerChan = closeCh
		consumes := func(data []byte, attrs interceptor.Attributes, err error) {
			me.consumerMu.Lock()
			defer me.consumerMu.Unlock()
			consumers := me.consumers
			for _, consumer := range consumers {
				stop := (*consumer)(me.remote.SSRC(), data, attrs, err)
				if stop {
					me.removeConsumer(consumer)
				}
			}
		}
		go func() {
			buffer := make([]byte, PACKET_MAX_SIZE)
			for {
				select {
				case <-closeCh:
					return
				default:
					n, attrs, err := me.remote.Read(buffer)
					me.pub.ctx.Metrics().OnWebrtcRtpRead(me.pub.ctx, n)
					if err != nil {
						consumes(nil, nil, err)
						break
					}
					data := buffer[:n]
					consumes(data, attrs, nil)
				}
			}
		}()
	}
	return
}

func (me *PublishedTrack) Labels() map[string]string {
	return me.track.Labels
}

func (me *PublishedTrack) GlobalId() string {
	return me.track.GlobalId
}

func (me *PublishedTrack) StreamId() string {
	return me.track.StreamId
}

func (me *PublishedTrack) Remote() *webrtc.TrackRemote {
	return me.remote
}

func (me *PublishedTrack) Rid() string {
	return me.track.Rid
}

func (me *PublishedTrack) BindId() string {
	return me.track.BindId
}

func (pt *PublishedTrack) isBind() bool {
	return pt.remote != nil
}

func (pt *PublishedTrack) Bind() bool {
	ctx := pt.pub.ctx
	peer, err := ctx.makeSurePeer()
	if err != nil {
		panic(err)
	}
	success := func() bool {
		pt.mu.Lock()
		defer pt.mu.Unlock()
		rt, t := ctx.findTracksRemoteByMidAndRid(peer, pt.StreamId(), pt.BindId(), pt.Rid())
		if rt != nil {
			if rt == pt.remote {
				return true
			} else if pt.remote != nil {
				ctx.Sugar().Debugf("rebind track, old track: %s/%s", pt.remote.ID(), pt.remote.Msid())
			}
			ctx.Sugar().Debugf("bind track with mid %s/%s", pt.BindId(), t.Mid())
			codec := rt.Codec()
			if codec.MimeType != "" {
				ctx.Sugar().Debugf("track mime type gotten: %s", codec.MimeType)
				pt.track.Codec = model.NewRTPCodecParameters(&codec)
			} else {
				ctx.Sugar().Debugf("no track mime type found, track with mid %s/%s bind failed", pt.BindId(), t.Mid())
				return false
			}
			ctx.Sugar().Debugf("track with mid %s/%s bind successfully", pt.BindId(), t.Mid())
			pt.remote = rt
			pt.track.LocalId = rt.ID()
			return true
		}
		return false
	}()
	if success {
		success = ctx.mustEmitWithAck(
			"published",
			"send published msg",
			&model.PublishedMessage{
				Track: pt.Track(),
			},
		) == nil
	}
	return success
}

func (me *PublishedTrack) SatifySelect(transportId string) bool {
	if me.pub.IsClosed() {
		return false
	}
	ctx := me.pub.ctx
	peer, err := ctx.makeSurePeer()
	if err != nil {
		panic(err)
	}
	me.mu.Lock()
	defer me.mu.Unlock()
	tr, _ := ctx.findTracksRemoteByMidAndRid(peer, me.StreamId(), me.BindId(), me.Rid())
	if tr != nil {
		ctx.Sugar().Debugf("Accepting track with codec ", tr.Codec().MimeType)
		r := me.pub.ctx.Global.Router()
		ch := r.PublishTrack(me.GlobalId(), transportId)
		var onRTP RTPConsumer = func(ssrc webrtc.SSRC, data []byte, attrs interceptor.Attributes, err error) bool {
			if err != nil {
				r.RemoveTrack(me.GlobalId())
				if IsRTPClosedError(err) {
					p, err := NewEOFPacket(me.GlobalId())
					if err != nil {
						panic(err)
					}
					ch <- p
					return true
				}
				panic(err)
			}
			select {
			case <-me.pub.closeCh:
				r.RemoveTrack(me.GlobalId())
				p, err := NewEOFPacket(me.GlobalId())
				if err != nil {
					panic(err)
				}
				ch <- p
				return true
			default:
				if len(data) > 0 {
					p, err := NewPacketFromBuf(me.GlobalId(), data)
					if err != nil {
						r.RemoveTrack(me.GlobalId())
						panic(err)
					}
					ch <- p
				}
				return false
			}
		}
		me.OnRTPPacket(&onRTP)
		return true
	} else {
		return false
	}
}

type RoomInfo struct {
	JoinId uint32
	Joined bool
}

type SignalContext struct {
	Id                string
	Global            *Global
	Signal            Signal
	AuthInfo          *auth.AuthInfo
	Peer              *webrtc.PeerConnection
	ctx               context.Context
	cancel            context.CancelCauseFunc
	rooms             map[string]RoomInfo
	rooms_mux         sync.RWMutex
	pendingCandidates []*model.CandidateMessage
	cand_mux          sync.Mutex
	peer_mux          sync.Mutex
	neg_mux           sync.Mutex
	inited            bool
	inited_mux        sync.Mutex
	closed            bool
	closed_mux        sync.Mutex
	close_cb_disabled bool
	close_cb_mux      sync.Mutex
	close_cb          *ConferenceCallback
	subscriptions     *haxmap.Map[string, *Subscription]
	publications      *haxmap.Map[string, *Publication]
	peerClosed        bool
	sdpMsgId          atomic.Int32
	logger            *zap.Logger
	sugar             *zap.SugaredLogger
}

func newSignalContext(global *Global, signal Signal, authInfo *auth.AuthInfo, id string) *SignalContext {
	logger := log.Logger().With(zap.String("tag", "signal"), zap.String("id", id))
	ctx, cancel := context.WithCancelCause(global.ctx)
	return &SignalContext{
		Global:        global,
		Id:            id,
		Signal:        signal,
		AuthInfo:      authInfo,
		ctx:           ctx,
		cancel:        cancel,
		rooms:         make(map[string]RoomInfo),
		subscriptions: haxmap.New[string, *Subscription](),
		publications:  haxmap.New[string, *Publication](),
		logger:        logger,
		sugar:         logger.Sugar(),
	}
}

func (ctx *SignalContext) Messager() *Messager {
	return ctx.Global.GetMessager()
}

func (ctx *SignalContext) Conf() *config.ConferenceConfigure {
	return ctx.Global.conf
}

func (ctx *SignalContext) Metrics() *Metrics {
	return ctx.Global.GetMetrics()
}

func (ctx *SignalContext) Logger() *zap.Logger {
	return ctx.logger
}

func (ctx *SignalContext) Sugar() *zap.SugaredLogger {
	return ctx.sugar
}

func (ctx *SignalContext) Authed() bool {
	return ctx.AuthInfo != nil
}

func (ctx *SignalContext) RoomPaterns() []string {
	return ctx.AuthInfo.Rooms
}

func (ctx *SignalContext) Rooms() []string {
	ctx.rooms_mux.RLock()
	defer ctx.rooms_mux.RUnlock()
	cap := len(ctx.rooms)
	rooms := make([]string, 0, cap)
	for room, info := range ctx.rooms {
		if info.Joined {
			rooms = append(rooms, room)
		}
	}
	return rooms
}

func (ctx *SignalContext) JoinId(room string) uint32 {
	ctx.rooms_mux.RLock()
	defer ctx.rooms_mux.RUnlock()
	info, ok := ctx.rooms[room]
	if ok {
		if info.Joined {
			return info.JoinId
		} else {
			return 0
		}
	} else {
		return 0
	}
}

func (ctx *SignalContext) NextSdpMsgId() int {
	return int(ctx.sdpMsgId.Add(2))
}

func (ctx *SignalContext) CurrentSdpMsgId() int {
	return int(ctx.sdpMsgId.Load())
}

func (sctx *SignalContext) clusterEmit(message model.RoomMessage) {
	router := message.GetRouter()
	if router != nil && router.GetRoom() != "" {
		room := router.GetRoom()
		msg_copy := message.CopyPlain()
		msg_copy.FixRouter(room, sctx.AuthInfo.UID, sctx.Messager().NodeName())
		err := sctx.Messager().Emit(sctx.ctx, msg_copy)
		if err != nil {
			sctx.Sugar().Error("cluster msg emit failed, %v", err)
		}
	} else {
		for _, room := range sctx.Rooms() {
			msg_copy := message.CopyPlain()
			msg_copy.FixRouter(room, sctx.AuthInfo.UID, sctx.Messager().NodeName())
			err := sctx.Messager().Emit(sctx.ctx, msg_copy)
			if err != nil {
				sctx.Sugar().Error("cluster msg emit failed, %v", err)
			}
		}
	}
}

func (sctx *SignalContext) ClusterEmit(message model.RoomMessage) error {
	sctx.clusterEmit(message)
	return nil
}

func (sctx *SignalContext) _emit(ack bool, ev string, args ...any) (res []any, err error) {
	timeout := time.Duration(sctx.Conf().Signal.MsgTimeoutMs) * time.Millisecond
	var ctx context.Context
	if timeout > 0 {
		ctx, _ = context.WithTimeoutCause(sctx.ctx, timeout, context.DeadlineExceeded)
	} else {
		ctx = sctx.ctx
	}
	if ack {
		retries := sctx.Conf().Signal.MsgTimeoutRetries
		if retries < 0 {
			retries = 0
		}
		for i := 0; i <= retries; i++ {
			res, err = sctx.Signal.SendMsg(ctx, ack, ev, args...)
			if err == nil {
				return
			}
		}
		return
	} else {
		return sctx.Signal.SendMsg(ctx, false, ev, args...)
	}
}

func (ctx *SignalContext) emit(ev string, args ...any) error {
	_, err := ctx._emit(false, ev, args...)
	return err
}

func (ctx *SignalContext) Emit(ev string, args ...any) error {
	return ctx.emit(ev, args...)
}

func (ctx *SignalContext) emitWithAck(ev string, args ...any) ([]any, error) {
	return ctx._emit(true, ev, args...)
}

func (ctx *SignalContext) EmitWithAck(ev string, args ...any) ([]any, error) {
	return ctx.emitWithAck(ev, args...)
}

func (ctx *SignalContext) mustEmitWithAck(ev string, cause string, args ...any) error {
	_, err := ctx.emitWithAck(ev, args...)
	if err != nil {
		ctx.Sugar().Errorf("send %s msg with args %v failed: %v", ev, args, err)
		FatalErrorAndClose(ctx, err, cause)
	}
	return err
}

func (ctx *SignalContext) MustEmitWithAck(ev string, cause string, args ...any) error {
	return ctx.mustEmitWithAck(ev, cause, args...)
}

func (ctx *SignalContext) SetCloseCallback(cb *ConferenceCallback) {
	ctx.close_cb_mux.Lock()
	defer ctx.close_cb_mux.Unlock()
	ctx.close_cb = cb
}

func (ctx *SignalContext) disableCloseCallback() {
	ctx.close_cb_mux.Lock()
	defer ctx.close_cb_mux.Unlock()
	ctx.close_cb_disabled = true
}

func (ctx *SignalContext) MakeUserInfo() *model.UserInfo {
	return &model.UserInfo{
		Key:      ctx.AuthInfo.Key,
		UserId:   ctx.AuthInfo.UID,
		UserName: ctx.AuthInfo.UName,
		Role:     ctx.AuthInfo.Role,
		Rooms:    ctx.Rooms(),
	}
}

func (sctx *SignalContext) Ready() error {
	return sctx.Emit("ready", sctx.MakeUserInfo())
}

func (ctx *SignalContext) Close() {
	ctx.Global.CloseSignalContext(ctx.Id, false)
}

func (ctx *SignalContext) SelfClose(disableCloseCallback bool) {
	ctx.Sugar().Debugf("closing the signal context")
	if ctx.closed {
		ctx.Sugar().Debugf("the signal context already closed, return directly")
		return
	}
	ctx.closed_mux.Lock()
	if ctx.closed {
		ctx.closed_mux.Unlock()
		ctx.Sugar().Debugf("the signal context already closed, return directly")
		return
	}
	ctx.closed = true
	defer ctx.closed_mux.Unlock()
	// At first, should leave rooms
	ctx.LeaveRoom(ctx.Rooms()...)
	ctx.cancel(nil)
	if disableCloseCallback {
		ctx.disableCloseCallback()
	}
	ctx.Messager().OffState(ctx.Id, ctx.RoomPaterns()...)
	ctx.Messager().OffWant(ctx.Id, ctx.RoomPaterns()...)
	ctx.Messager().OffSelect(ctx.Id, ctx.RoomPaterns()...)
	ctx.Messager().OffWantParticipant(ctx.Id, ctx.RoomPaterns()...)
	ctx.Messager().OffStateParticipant(ctx.Id, ctx.RoomPaterns()...)
	ctx.Messager().OffStateLeave(ctx.Id, ctx.RoomPaterns()...)
	ctx.Messager().OffPing(ctx.Id, ctx.RoomPaterns()...)
	ctx.Messager().OffPong(ctx.Id, ctx.RoomPaterns()...)
	ctx.Messager().OffCustom(ctx.Id, ctx.RoomPaterns()...)
	ctx.Messager().OffCustomAck(ctx.Id, ctx.RoomPaterns()...)
	ctx.Sugar().Debugf("signal context closing")
	ctx.closePeer()
	ctx.publications.ForEach(func(k string, pub *Publication) bool {
		pub.Close()
		return true
	})
	ctx.subscriptions.ForEach(func(k string, sub *Subscription) bool {
		sub.Close()
		return true
	})
	ctx.Signal.Close()
	ctx.close_cb_mux.Lock()
	defer ctx.close_cb_mux.Unlock()
	if !ctx.close_cb_disabled && ctx.close_cb != nil {
		cb := ctx.close_cb
		ctx.close_cb = nil
		go cb.Call(ctx)
	}
	ctx.Sugar().Debugf("the signal context closed")
}

func (ctx *SignalContext) matchRoom(msg model.RoomMessage) bool {
	router := msg.GetRouter()
	if router == nil {
		return false
	}
	room := router.GetRoom()
	if room == "" {
		return false
	}
	id := ctx.JoinId(room)
	return id != 0
}

func (ctx *SignalContext) matchUserTo(msg model.RoomMessage, strict bool) bool {
	router := msg.GetRouter()
	if router == nil {
		return false
	}
	userTo := router.GetUserTo()
	if strict {
		return userTo == ctx.AuthInfo.UID
	} else {
		return userTo == "" || userTo == ctx.AuthInfo.UID
	}
}

func (ctx *SignalContext) AcceptTrack(msg *model.StateMessage) {
	if !ctx.matchRoom(msg) {
		return
	}
	ctx.subscriptions.ForEach(func(k string, sub *Subscription) bool {
		sub.AcceptTrack(msg)
		return true
	})
}

func findTransiverBySender(peer *webrtc.PeerConnection, sender *webrtc.RTPSender) (*webrtc.RTPTransceiver, int) {
	if transceivers := peer.GetTransceivers(); transceivers != nil {
		for i, transceiver := range transceivers {
			if transceiver.Sender() == sender {
				return transceiver, i
			}
		}
	}
	return nil, -1
}

func getMidFromSender(peer *webrtc.PeerConnection, sender *webrtc.RTPSender) string {
	transceiver, pos := findTransiverBySender(peer, sender)
	if transceiver != nil {
		if transceiver.Mid() != "" {
			return transceiver.Mid()
		} else {
			return fmt.Sprintf("pos:%d", pos)
		}
	} else {
		return ""
	}
}

func (ctx *SignalContext) Subscribe(message *model.SubscribeMessage) (subId string, err error) {
	err = message.Validate()
	if err != nil {
		return
	}
	switch message.Op {
	case model.SUB_OP_ADD:
		var loaded = true
		for loaded {
			subId = uuid.NewString()
			_, loaded = ctx.subscriptions.GetOrCompute(subId, func() *Subscription {
				return &Subscription{
					id:       subId,
					reqTypes: message.ReqTypes,
					pattern:  message.Pattern,
					sctx:     ctx,
				}
			})
		}
	case model.SUB_OP_UPDATE:
		sub, ok := ctx.subscriptions.Get(message.Id)
		if !ok {
			err = errors.SubNotExist(message.Id)
			return
		}
		sub.UpdatePattern(message)
		subId = sub.id
	case model.SUB_OP_REMOVE:
		sub, ok := ctx.subscriptions.GetAndDel(message.Id)
		if !ok {
			err = errors.SubNotExist(message.Id)
			return
		}
		sub.Close()
		subId = sub.id
		return
	default:
		panic(errors.ThisIsImpossible().GenCallStacks(0))
	}

	r := ctx.Global.Router()
	ctx.clusterEmit(&model.WantMessage{
		Router: &model.RouterMessage{
			Room: message.Room,
		},
		Pattern:     message.Pattern,
		TransportId: r.id.String(),
	})
	return
}

func (ctx *SignalContext) findTracksRemoteByMidAndRid(peer *webrtc.PeerConnection, sid string, mid string, rid string) (*webrtc.TrackRemote, *webrtc.RTPTransceiver) {
	var pos int = -1
	var err error
	if strings.HasPrefix(mid, "pos:") {
		pos, err = strconv.Atoi(mid[4:])
		if err != nil {
			panic(err)
		}
	}
	for i, transceiver := range peer.GetTransceivers() {
		if (pos == -1 && transceiver.Mid() == mid) || pos == i {
			r := transceiver.Receiver()
			if r != nil {
				tracks := r.Tracks()
				if len(tracks) > 1 {
					for _, t := range tracks {
						if t.RID() == rid {
							if t.StreamID() == sid {
								return t, transceiver
							} else {
								return nil, transceiver
							}
						}
					}
					return nil, transceiver
				} else if len(tracks) == 0 {
					return nil, transceiver
				} else {
					track := tracks[0]
					if rid != "" {
						if track.RID() == rid {
							if track.StreamID() == sid {
								return track, transceiver
							} else {
								return nil, transceiver
							}
						} else {
							return nil, transceiver
						}
					} else {
						if track.StreamID() == sid {
							return track, transceiver
						} else {
							return nil, transceiver
						}
					}
				}
			} else {
				return nil, transceiver
			}
		}
	}
	return nil, nil
}

func (ctx *SignalContext) Publish(message *model.PublishMessage) (pubId string, err error) {
	err = message.Validate()
	if err != nil {
		return
	}
	switch message.Op {
	case model.PUB_OP_ADD:
		var loaded bool = true
		for loaded {
			pubId = uuid.NewString()
			_, loaded = ctx.publications.GetOrCompute(pubId, func() *Publication {
				pub := &Publication{
					id:      pubId,
					ctx:     ctx,
					tracks:  make(map[string]*PublishedTrack),
					closeCh: make(chan struct{}),
				}
				for _, t := range message.Tracks {
					tid := uuid.NewString()
					pub.tracks[tid] = &PublishedTrack{
						pub: pub,
						track: &model.Track{
							Type:     t.Type,
							PubId:    pubId,
							GlobalId: tid,
							BindId:   t.BindId,
							Rid:      t.Rid,
							StreamId: t.Sid,
							Labels:   t.Labels,
						},
					}
				}
				return pub
			})
		}
	case model.PUB_OP_REMOVE:
		pub, ok := ctx.publications.Get(message.Id)
		if ok {
			pub.Close()
			pubId = message.Id
		}
	default:
		panic(errors.ThisIsImpossible().GenCallStacks(0))
	}
	// var pub *Publication
	// pub.Bind()
	return
}

func (ctx *SignalContext) StateWant(message *model.WantMessage) {
	if !ctx.matchRoom(message) {
		return
	}
	r := ctx.Global.Router()
	ctx.publications.ForEach(func(pubId string, pub *Publication) bool {
		satified := pub.State(message)
		if len(satified) > 0 {
			ctx.clusterEmit(&model.StateMessage{
				PubId:  pub.id,
				Tracks: satified,
				Addr:   r.Addr(),
			})
		}
		return true
	})
}

func (ctx *SignalContext) SatifySelect(message *model.SelectMessage) {
	if !ctx.matchRoom(message) {
		return
	}
	pub, found := ctx.publications.Get(message.PubId)
	if found {
		pub.SatifySelect(message)
	}
}

func (ctx *SignalContext) StateParticipants(message *model.WantParticipantMessage) {
	room := proto.GetRoom(message)
	if room == "" {
		return
	}
	joinId := ctx.JoinId(room)
	if joinId == 0 {
		return
	}
	ctx.clusterEmit(&model.StateParticipantMessage{
		UserId:   ctx.AuthInfo.UID,
		UserName: ctx.AuthInfo.UName,
		SocketId: ctx.Id,
		JoinId:   joinId,
	})
}

func (ctx *SignalContext) AcceptParticipants(message *model.StateParticipantMessage) {
	if !ctx.matchRoom(message) {
		return
	}
	ctx.emit("participant-join", proto.ToClientMessage(message))
}

func (ctx *SignalContext) StateParticipantLeave(message *model.StateLeaveMessage) {
	if !ctx.matchRoom(message) {
		return
	}
	ctx.emit("participant-leave", proto.ToClientMessage(message))
}

func (ctx *SignalContext) Bind() {
	ctx.publications.ForEach(func(pid string, pub *Publication) bool {
		if pub.Bind() {
			return false
		} else {
			return true
		}
	})
}

func (ctx *SignalContext) OnPingMessage(message *model.PingMessage) {
	if !ctx.matchRoom(message) || !ctx.matchUserTo(message, true) {
		return
	}
	ctx.emit("ping", proto.ToClientMessage(message))
}

func (ctx *SignalContext) OnPongMessage(message *model.PongMessage) {
	if !ctx.matchRoom(message) || !ctx.matchUserTo(message, true) {
		return
	}
	ctx.emit("pong", proto.ToClientMessage(message))
}

func (ctx *SignalContext) OnCustomMessage(message *model.CustomClusterMessage) {
	if !ctx.matchRoom(message) || !ctx.matchUserTo(message, false) {
		return
	}
	ctx.emit(fmt.Sprintf("custom:%s", message.Evt), proto.ToClientMessage(message.Msg))
}

func (ctx *SignalContext) OnCustomAckMessage(message *model.CustomAckMessage) {
	if !ctx.matchRoom(message) || !ctx.matchUserTo(message, true) {
		return
	}
	ctx.emit("custom-ack", proto.ToClientMessage(message))
}

func (ctx *SignalContext) closePeer() {
	if ctx.peerClosed {
		return
	}
	ctx.peer_mux.Lock()
	defer ctx.peer_mux.Unlock()
	if ctx.peerClosed {
		return
	}
	ctx.peerClosed = true
	if ctx.Peer != nil {
		ctx.Peer.OnICECandidate(nil)
		ctx.Peer.OnTrack(nil)
		ctx.Peer.OnConnectionStateChange(nil)
		ctx.Peer.OnICEConnectionStateChange(nil)
		ctx.Peer.OnICEGatheringStateChange(nil)
		ctx.Peer.OnNegotiationNeeded(nil)
		ctx.Peer.OnSignalingStateChange(nil)
		// ignore error
		ctx.Peer.Close()
	}
}

func (ctx *SignalContext) StartNegotiate(peer *webrtc.PeerConnection, msgId int) (err error) {
	ctx.Sugar().Debug("neg mux locked")
	ctx.neg_mux.Lock()
	offer, err := peer.CreateOffer(nil)
	if err != nil {
		return
	}
	err = peer.SetLocalDescription(offer)
	if err != nil {
		return
	}
	desc := peer.LocalDescription()
	return ctx.mustEmitWithAck("sdp", "send sdp msg", model.SdpMessage{
		Type: desc.Type.String(),
		Sdp:  desc.SDP,
		Mid:  msgId,
	})
}

func MakeRTPCodecCapability(mimeType string, clockRate uint32, channels uint16, sdpFmtpLine string, feedback []webrtc.RTCPFeedback) webrtc.RTPCodecCapability {
	return webrtc.RTPCodecCapability{
		MimeType:     mimeType,
		ClockRate:    clockRate,
		Channels:     channels,
		SDPFmtpLine:  sdpFmtpLine,
		RTCPFeedback: feedback,
	}
}

func RegisterLeastCodecs(m *webrtc.MediaEngine) error {
	// Default Pion Audio Codecs
	for _, codec := range []webrtc.RTPCodecParameters{
		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypeOpus, 48000, 2, "minptime=10;useinbandfec=1", nil),
			PayloadType:        111,
		},
		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypeG722, 8000, 0, "", nil),
			PayloadType:        9,
		},
		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypePCMU, 8000, 0, "", nil),
			PayloadType:        0,
		},
		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypePCMA, 8000, 0, "", nil),
			PayloadType:        8,
		},
	} {
		if err := m.RegisterCodec(codec, webrtc.RTPCodecTypeAudio); err != nil {
			return err
		}
	}
	videoRTCPFeedback := []webrtc.RTCPFeedback{
		{
			Type:      webrtc.TypeRTCPFBGoogREMB,
			Parameter: "",
		},
		{
			Type:      webrtc.TypeRTCPFBCCM,
			Parameter: "fir",
		},
		{
			Type:      webrtc.TypeRTCPFBNACK,
			Parameter: "",
		},
		{
			Type:      webrtc.TypeRTCPFBNACK,
			Parameter: "pli",
		},
	}
	for _, codec := range []webrtc.RTPCodecParameters{

		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f", videoRTCPFeedback),
			PayloadType:        102,
		},
		{
			RTPCodecCapability: MakeRTPCodecCapability("video/rtx", 90000, 0, "apt=102", nil),
			PayloadType:        103,
		},

		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42001f", videoRTCPFeedback),
			PayloadType:        104,
		},
		{
			RTPCodecCapability: MakeRTPCodecCapability("video/rtx", 90000, 0, "apt=104", nil),
			PayloadType:        105,
		},

		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", videoRTCPFeedback),
			PayloadType:        106,
		},
		{
			RTPCodecCapability: MakeRTPCodecCapability("video/rtx", 90000, 0, "apt=106", nil),
			PayloadType:        107,
		},

		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42e01f", videoRTCPFeedback),
			PayloadType:        108,
		},
		{
			RTPCodecCapability: MakeRTPCodecCapability("video/rtx", 90000, 0, "apt=108", nil),
			PayloadType:        109,
		},

		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=4d001f", videoRTCPFeedback),
			PayloadType:        127,
		},
		{
			RTPCodecCapability: MakeRTPCodecCapability("video/rtx", 90000, 0, "apt=127", nil),
			PayloadType:        125,
		},

		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=4d001f", videoRTCPFeedback),
			PayloadType:        39,
		},
		{
			RTPCodecCapability: MakeRTPCodecCapability("video/rtx", 90000, 0, "apt=39", nil),
			PayloadType:        40,
		},

		{
			RTPCodecCapability: MakeRTPCodecCapability(webrtc.MimeTypeH264, 90000, 0, "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=64001f", videoRTCPFeedback),
			PayloadType:        112,
		},
		{
			RTPCodecCapability: MakeRTPCodecCapability("video/rtx", 90000, 0, "apt=112", nil),
			PayloadType:        113,
		},
	} {
		if err := m.RegisterCodec(codec, webrtc.RTPCodecTypeVideo); err != nil {
			return err
		}
	}

	return nil
}

func createPeer(conf *config.ConferenceConfigure) (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}
	if conf.Record.Enable {
		if err := RegisterLeastCodecs(m); err != nil {
			return nil, err
		}
	} else {
		if err := m.RegisterDefaultCodecs(); err != nil {
			// if err := RegisterLeastCodecs(m); err != nil {
			return nil, err
		}
	}
	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, err
	}
	// Register a intervalpli factory
	// This interceptor sends a PLI every 3 seconds. A PLI causes a video keyframe to be generated by the sender.
	// This makes our video seekable and more error resilent, but at a cost of lower picture quality and higher bitrates
	// A real world application should process incoming RTCP packets from viewers and forward them to senders
	intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		panic(err)
	}
	i.Add(intervalPliFactory)
	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))
	config := conf.GetWebrtcConfiguration()
	// Create a new RTCPeerConnection
	peer, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}
	return peer, err
}

func (ctx *SignalContext) makeSurePeer() (peer *webrtc.PeerConnection, err error) {
	if ctx.peerClosed {
		err = webrtc.ErrConnectionClosed
		return
	}
	ctx.peer_mux.Lock()
	defer ctx.peer_mux.Unlock()
	if ctx.peerClosed {
		err = webrtc.ErrConnectionClosed
		return
	}
	if ctx.Peer != nil {
		peer = ctx.Peer
	} else {
		peer, err = createPeer(ctx.Global.Conf())
		if err != nil {
			return
		}
		peer.OnICECandidate(func(i *webrtc.ICECandidate) {
			if i == nil {
				ctx.Sugar().Debug("can not find candidate any more")
				ctx.MustEmitWithAck("candidate", "send candidate msg", &model.CandidateMessage{
					Op: "end",
				})
			} else {
				ctx.Sugar().Debugf("find candidate %v", i)
				ctx.MustEmitWithAck("candidate", "send candidate msg", &model.CandidateMessage{
					Op:        "add",
					Candidate: i.ToJSON(),
				})
			}
		})
		peer.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
			ctx.Sugar().Debugf("accept track with mid %s, mime type: %v, playload type: %v", r.RTPTransceiver().Mid(), r.Track().Codec().MimeType, r.Track().Codec().PayloadType)
			ctx.Bind()
		})

		lastState := peer.ConnectionState()
		peer.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
			ctx.Sugar().Info("peer connect state changed from ", lastState, " to ", pcs)
			lastState = pcs
			if pcs == webrtc.PeerConnectionStateConnected {
				ctx.Metrics().OnWebrtcConnectStart(ctx)
			} else if pcs == webrtc.PeerConnectionStateClosed || pcs == webrtc.PeerConnectionStateFailed {
				ctx.Metrics().OnWebrtcConnectClose(ctx)
				ctx.Close()
			}
		})
		lastIceConnectionState := peer.ICEConnectionState()
		peer.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
			ctx.Sugar().Debugf("peer ice connection state changed from %v to %v", lastIceConnectionState, is)
			lastIceConnectionState = is
		})
		lastIceGatheringState := peer.ICEGatheringState()
		peer.OnICEGatheringStateChange(func(is webrtc.ICEGatheringState) {
			ctx.Sugar().Debugf("peer ice gathering state changed from %v to %v", lastIceGatheringState, is)
			lastIceGatheringState = is
		})
		lastSignalingState := peer.SignalingState()
		peer.OnSignalingStateChange(func(ss webrtc.SignalingState) {
			ctx.Sugar().Info("peer signaling state changed from ", lastSignalingState, " to ", ss)
			lastSignalingState = ss
			if ss == webrtc.SignalingStateStable {
				ctx.neg_mux.TryLock()
				ctx.Sugar().Debug("neg mux unlocked")
				ctx.neg_mux.Unlock()
			}
		})
		ctx.Peer = peer
	}
	return
}

func (ctx *SignalContext) MakeSurePeer() (peer *webrtc.PeerConnection, err error) {
	return ctx.makeSurePeer()
}

func (ctx *SignalContext) hasRoomRight(room string) bool {
	for _, p := range ctx.RoomPaterns() {
		if MatchRoom(p, room) {
			return true
		}
	}
	return false
}

func (ctx *SignalContext) JoinRoom(rooms ...string) error {
	var s_rooms []string
	if len(rooms) == 0 {
		for _, p := range ctx.RoomPaterns() {
			if !strings.Contains(p, "*") {
				s_rooms = append(s_rooms, p)
			}
		}
		if len(s_rooms) == 0 {
			return errors.InvalidParam("room is not provided")
		}
	} else {
		for _, room := range rooms {
			if !ctx.hasRoomRight(room) {
				return errors.RoomNoRight(room)
			}
		}
		s_rooms = rooms
	}
	joined_rooms := make([]string, 0, len(s_rooms))
	func() {
		ctx.rooms_mux.Lock()
		defer ctx.rooms_mux.Unlock()
		for _, room := range s_rooms {
			info, ok := ctx.rooms[room]
			if ok {
				if !info.Joined {
					info.JoinId++
					info.Joined = true
					joined_rooms = append(joined_rooms, room)
				}
			} else {
				ctx.rooms[room] = RoomInfo{
					JoinId: 1,
					Joined: true,
				}
				joined_rooms = append(joined_rooms, room)
			}
		}
	}()
	for _, room := range joined_rooms {
		joinId := ctx.JoinId(room)
		if joinId != 0 {
			ctx.ClusterEmit(&model.StateParticipantMessage{
				Router: &model.RouterMessage{
					Room: room,
				},
				UserId:   ctx.AuthInfo.UID,
				UserName: ctx.AuthInfo.UName,
				SocketId: ctx.Id,
				JoinId:   joinId,
			})
		}
	}
	return nil
}

func (ctx *SignalContext) LeaveRoom(rooms ...string) {
	leaved := func() map[string]uint32 {
		ctx.rooms_mux.Lock()
		defer ctx.rooms_mux.Unlock()
		removed := make(map[string]uint32)
		for _, room := range rooms {
			info, ok := ctx.rooms[room]
			if ok && info.Joined {
				info.Joined = false
				removed[room] = info.JoinId
			}
		}
		return removed
	}()
	for room, joinId := range leaved {
		ctx.ClusterEmit(&model.StateLeaveMessage{
			Router: &model.RouterMessage{
				Room: room,
			},
			UserId:   ctx.AuthInfo.UID,
			SocketId: ctx.Id,
			JoinId:   joinId,
		})
	}
}
