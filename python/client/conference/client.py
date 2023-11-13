from dataclasses import dataclass, field
import asyncio as aio
import time 

from typing import Any, Callable, Generic, TypeVar
import socketio as skt
from aiortc import RTCPeerConnection, RTCConfiguration

from conference.utils import splitUrl
from conference.stream import LocalStream
from conference.pattern import Pattern
from conference.messages import SignalMessage, SubscribeAddMessage, SubscribeResultMessage, SubscribedMessage

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

class ConferenceClient():
    conf: Configuration
    peer: RTCPeerConnection
    io: skt.AsyncClient
    io_connect_evt: aio.Event
    ark_lock: aio.Lock
    pending_subscribed_msgs: dict[str, MsgBox[SubscribedMessage]]
    subscribed_msg_callbacks: dict[str, Callable[[SubscribedMessage], None]]
    
    def __init__(self, conf: Configuration) -> None:
        self.conf = conf
        self.peer = RTCPeerConnection(conf.rtcConfig)
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
        self.io_connect_evt = aio.Event()
        self.ark_lock = aio.Lock()
        self.pending_subscribed_msgs = {}
        self.subscribed_msg_callbacks = {}
        
    def __prepare_peer(self):
        pass
        
    async def __makesure_socket_connect(self):
        if self.io_connect_evt.is_set():
            return
        host, path = splitUrl(self.conf.signalUrl)
        await self.io.connect(url=host, socketio_path=path, auth=self.conf.token)
        self.io.on("subscribed", self.__on_subscribed)
        self.io_connect_evt.set()
        
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
            
    def __catch_subscribed(self, subId: str, future: aio.Future[SubscribedMessage]):
        def callback(msg: SubscribedMessage):
            future.set_result(msg)
        msg = self.pending_subscribed_msgs.get(subId)
        if msg is not None:
            del self.pending_subscribed_msgs[subId]
            callable(msg)
        else:  
            self.subscribed_msg_callbacks[subId] = callback
            
    async def __emit_subscribe(self, msg: SubscribeAddMessage):
        async with self.ark_lock:
            sr_fut: aio.Future[SubscribeResultMessage] = aio.Future()
            def subscribe_ark(msg: dict):
                sr_fut.set_result(SubscribeResultMessage(**msg))
            await self.io.emit('subscribe', msg, callback=subscribe_ark)
            res_msg = await sr_fut
        subscribed_fut: aio.Future[SubscribedMessage] = aio.Future()
        self.__catch_subscribed(res_msg.id, subscribed_fut)
        subscribed_msg = await subscribed_fut
        return subscribed_msg
        
    async def __close_socket_connect(self):
        if not self.io_connect_evt.is_set():
            assert not self.io.connected
            return
        await self.io.disconnect()
        self.io_connect_evt.clear()
        
    async def subscribe(self, pattern: Pattern, reqTypes: list[str] | None = None):
        await self.__makesure_socket_connect()
        subscribed_msg = self.__emit_subscribe(SubscribeAddMessage(pattern=pattern, reqTypes=reqTypes))
            
        