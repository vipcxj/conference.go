from dataclasses import dataclass
from typing import Literal
from enum import IntEnum
from conference.pattern import Pattern
from conference.common import Track

class SubOp(IntEnum):
    ADD = 0
    UPDATE = 1
    REMOVE = 2

@dataclass(kw_only=True)
class SignalMessage:
    pass

@dataclass(kw_only=True)
class JoinMessage(SignalMessage):
    rooms: list[str] | None = None

@dataclass(kw_only=True)
class SdpMessage(SignalMessage):
    type: Literal['answer', 'offer', 'pranswer', 'rollback']
    sdp: str
    msgId: int
    
@dataclass(kw_only=True)
class SubscribeAddMessage(SignalMessage):
    op: Literal[SubOp.ADD] = SubOp.ADD
    reqTypes: list[str] | None = None
    pattern: Pattern
    
@dataclass(kw_only=True)
class SubscribeResultMessage(SignalMessage):
    id: str
    
@dataclass(kw_only=True)
class SubscribedMessage(SignalMessage):
    subId: str
    pubId: str
    sdpId: int
    tracks: list[Track]