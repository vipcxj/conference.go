from dataclasses import dataclass, field
import asyncio as aio
import time 

from typing import Any, Callable, Generic, TypeVar
from uu import Error
import socketio as skt
from aiortc import RTCPeerConnection, RTCConfiguration, RTCSessionDescription, MediaStreamTrack, RTCRtpTransceiver

from conference.utils import splitUrl
from conference.pattern import Pattern
from conference.messages import SdpMessage, SignalMessage, SubscribeAddMessage, SubscribeResultMessage, SubscribedMessage, Track

@dataclass
class SocketConfigure:
    reconnection: bool = True
    reconnection_attempts: int = 0
    reconnection_delay: int = 1
    reconnection_delay_max: int = 5
    randomization_factor: float = 0.5
    logger: bool = False
    serializer: str = 'default'
    json: Any | None = None
    handle_sigint: bool = True
    ssl_verify: bool = False
    kwargs: dict[str, Any] = field(default_factory=dict)

@dataclass    
class SubscribedTrack:
    metadata: Track
    track: MediaStreamTrack | None = None
    
def valid_track(track: MediaStreamTrack | None) -> MediaStreamTrack | None:
    if track is None or track.readyState == 'ended':
        return None
    else:
        return track
    
def resolve_bindId(bindId: str, peer: RTCPeerConnection) -> tuple[str | None, int, MediaStreamTrack | None]:
    index = -1
    if bindId.startswith('pos:'):
        index = int(bindId[4:])
    if index >= 0:
        t = peer.getTransceivers()[index]
        return t.mid, index, valid_track(t.receiver.track)
    else:
        for i, t in enumerate(peer.getTransceivers()):
            if t.mid == bindId:
                return bindId, i, valid_track(t.receiver.track)
        return None, -1, None
    
def check_track(track: MediaStreamTrack, peer: RTCPeerConnection) -> tuple[str | None, int]:
    for i, t in enumerate(peer.getTransceivers()):
        if t.receiver.track == track:
            return t.mid, i
    return None, -1

class SubscribedTracks:
    tracks1: dict[str, SubscribedTrack]
    tracks2: dict[int, SubscribedTrack]
    completes: int
    event: aio.Event
    
    def __init__(self, tracks: list[Track], peer: RTCPeerConnection) -> None:
        self.completes = 0
        self.event = aio.Event()
        for track in tracks:
            bindId, index, t = resolve_bindId(track.bindId, peer)
            st = SubscribedTrack(track, t)
            if bindId:
                self.tracks1[bindId] = st
            elif index != -1:
                self.tracks2[index] = st
            else:
                raise Error(f'Invalid bind id {track.bindId}.')
            if t is not None:
                self.completes += 1
        if self.is_complete():
            self.event.set()
        
    def is_complete(self) -> bool:
        return self.completes == len(self.tracks)
    
    @property
    def tracks(self):
        tracks: list[SubscribedTrack] = []
        for k in self.tracks1:
            tracks.append(self.tracks1[k])
        for k in self.tracks2:
            tracks.append(self.tracks2[k])
        return tracks
                
    def accept_track(self, track: MediaStreamTrack, peer: RTCPeerConnection) -> bool:
        found = False
        bindId, index = check_track(track, peer)
        if bindId:
            st = self.tracks1.get(bindId)
            if st is not None:
                if st.track is None:
                    st.track = track
                    self.completes += 1
                    found = True
                else:
                    raise Error(f'The track is added again.')
            elif index >= 0:
                st = self.tracks2.get(index)
                if st is not None:
                    del self.tracks2[index]
                    self.tracks1[bindId] = st
                    if st.track is None:
                        st.track = track
                        self.completes += 1
                        found = True
                    else:
                        raise Error(f'The track is added again.')
        elif index >= 0:
            st = self.tracks2.get(index)
            if st is not None:
                if st.track is None:
                    st.track = track
                    self.completes += 1
                    found = True
                else:
                    raise Error(f'The track is added again.')
        if found and self.is_complete():
            self.event.set()
        return self.is_complete()
            
    


@dataclass
class Configuration:
    signalUrl: str
    token: str
    rtcConfig: RTCConfiguration | None = None
    socketConfig: SocketConfigure | None = None
    name: str | None = None
    
MT = TypeVar('MT', bound=SignalMessage)
    
@dataclass
class MsgBox(Generic[MT]):
    msg: MT
    timestamp: float
    
class TrackBox:
    track: MediaStreamTrack
    timestamp: float
class ConferenceClient():
    conf: Configuration
    peer: RTCPeerConnection
    io: skt.AsyncClient
    io_initializing_evt: aio.Event
    io_ready_future: aio.Future[None]
    ark_lock: aio.Lock
    sub_lock: aio.Lock
    pending_subscribed_msgs: dict[str, MsgBox[SubscribedMessage]]
    subscribed_msg_callbacks: dict[str, Callable[[SubscribedMessage], None]]
    pending_sdp_msgs: dict[int, MsgBox[SdpMessage]]
    sdp_msg_callbacks: dict[int, Callable[[SdpMessage], None]]
    pending_tracks: list[TrackBox]
    tracks_callbacks: list[Callable[[MediaStreamTrack], bool]]
    
    def __init__(self, conf: Configuration) -> None:
        self.conf = conf
        self.peer = RTCPeerConnection(conf.rtcConfig)
        self.peer.on('track', self.__on_track)
        socketConfig = conf.socketConfig if conf.socketConfig is not None else SocketConfigure()
        self.io = skt.AsyncClient(
            reconnection=socketConfig.reconnection,
            reconnection_attempts=socketConfig.reconnection_attempts,
            reconnection_delay=socketConfig.reconnection_delay,
            reconnection_delay_max=socketConfig.reconnection_delay_max,
            randomization_factor=socketConfig.randomization_factor,
            logger=socketConfig.logger,
            serializer=socketConfig.serializer,
            json=socketConfig.json,
            handle_sigint=socketConfig.handle_sigint,
            ssl_verify=socketConfig.ssl_verify,
            **socketConfig.kwargs,
        )
        self.io_initializing_evt = aio.Event()
        self.io_ready_future = aio.Future()
        self.ark_lock = aio.Lock()
        self.sub_lock = aio.Lock()
        self.pending_subscribed_msgs = {}
        self.subscribed_msg_callbacks = {}
        self.pending_sdp_msgs = {}
        self.sdp_msg_callbacks = {}
        self.sdp_msg_id = 0
        self.pending_tracks = []
        self.tracks_callbacks = []
    
    async def __negotiate(self, sdpId: int, active: bool = False):
        if active:
            offer = await self.peer.createOffer()
            await self.peer.setLocalDescription(offer)
            offer = self.peer.localDescription
            await self.io.emit('sdp', SdpMessage(
                msgId=sdpId,
                type='offer',
                sdp=offer.sdp,
            ))
            answer_msg = await self.__catch_sdp(sdpId)
            if answer_msg.type != "answer":
                raise Error(f"Expect an answer msg with msg id {sdpId}, but received a {answer_msg.type} msg.")
            await self.peer.setRemoteDescription(RTCSessionDescription(
                sdp=answer_msg.sdp,
                type=answer_msg.type,
            ))
        else:
            offer_msg = await self.__catch_sdp(sdpId)
            if offer_msg.type != "offer":
                raise Error(f"Expect an offer msg with id {sdpId}, but received a {offer_msg.type} msg.")
            await self.peer.setRemoteDescription(RTCSessionDescription(
                sdp=offer_msg.sdp,
                type=offer_msg.type,
            ))
            answer = await self.peer.createAnswer()
            if answer is None:
                raise Error("Unable to create the answer.")
            await self.io.emit('sdp', SdpMessage(
                msgId=sdpId,
                type='answer',
                sdp=answer.sdp,
            ))
            await self.peer.setLocalDescription(answer)
        
    async def __makesure_socket_connect(self):
        if self.io_initializing_evt.is_set():
            await self.io_ready_future
            return
        self.io_initializing_evt.set()
        try:
            host, path = splitUrl(self.conf.signalUrl)
            await self.io.connect(url=host, socketio_path=path, auth=self.conf.token)
            self.io.on("subscribed", self.__on_subscribed)
            self.io.on('sdp', self.__on_sdp)
            self.io_ready_future.set_result(None)
        except BaseException as e:
            self.io_ready_future.set_exception(e)
            raise e

    def __on_track(self, track: MediaStreamTrack):
        self.tracks_callbacks = [cb for cb in self.tracks_callbacks if not cb(track)]
        
    
    def __catch_tracks(self, subscribedTracks: SubscribedTracks):
        def callback(track: MediaStreamTrack) -> bool:
            return subscribedTracks.accept_track(track, self.peer)
        self.tracks_callbacks.append(callback)
        
    def __on_sdp(self, raw_msg: dict):
        msg = SdpMessage(**raw_msg)
        callback = self.sdp_msg_callbacks.get(msg.msgId)
        if callback is not None:
            try:
                callback(msg)
            finally:
                del self.sdp_msg_callbacks[msg.msgId]
        else:
            self.pending_sdp_msgs[msg.msgId] = MsgBox(msg=msg, timestamp=time.time())
            
    def __catch_sdp(self, msgId: int):
        future: aio.Future[SdpMessage] = aio.Future()
        def callback(msg: SdpMessage):
            future.set_result(msg)
        msg = self.pending_sdp_msgs[msgId]
        if msg is not None:
            del self.pending_sdp_msgs[msgId]
            callback(msg.msg)
        else:
            self.sdp_msg_callbacks[msgId] = callback
        return future
        
    def __on_subscribed(self, raw_msg: dict):
        msg = SubscribedMessage(**raw_msg)
        callback = self.subscribed_msg_callbacks.get(msg.subId)
        if callback is not None:
            try:
                callback(msg)
            finally:
                del self.subscribed_msg_callbacks[msg.subId]
        else:
            self.pending_subscribed_msgs[msg.subId] = MsgBox(msg=msg, timestamp=time.time())
            
    def __catch_subscribed(self, subId: str):
        future: aio.Future[SubscribedMessage] = aio.Future()
        def callback(msg: SubscribedMessage):
            future.set_result(msg)
        msg = self.pending_subscribed_msgs.get(subId)
        if msg is not None:
            del self.pending_subscribed_msgs[subId]
            callback(msg.msg)
        else:  
            self.subscribed_msg_callbacks[subId] = callback
        return future
            
    async def __emit_subscribe(self, msg: SubscribeAddMessage):
        async with self.ark_lock:
            sr_fut: aio.Future[SubscribeResultMessage] = aio.Future()
            def subscribe_ark(msg: dict):
                sr_fut.set_result(SubscribeResultMessage(**msg))
            await self.io.emit('subscribe', msg, callback=subscribe_ark)
            res_msg = await sr_fut
        return await self.__catch_subscribed(res_msg.id)
        
    async def close(self):
        await self.peer.close()
        if self.io_initializing_evt.is_set():
            try:
                await self.io_ready_future
            except:
                pass
            await self.io.disconnect()
        
    async def subscribe(self, pattern: Pattern, reqTypes: list[str] | None = None):
        await self.__makesure_socket_connect()
        async with self.sub_lock:
            subscribed_msg = await self.__emit_subscribe(SubscribeAddMessage(
                pattern=pattern,
                reqTypes=reqTypes,
            ))
            subscribed_tracks = SubscribedTracks(tracks=subscribed_msg.tracks, peer=self.peer)
            self.__catch_tracks(subscribed_tracks)
            await self.__negotiate(subscribed_msg.sdpId)
            await subscribed_tracks.event.wait()
            return subscribed_tracks.tracks
            
      
        
            
        