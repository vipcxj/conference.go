from dataclasses import dataclass
from typing import Literal
from enum import IntEnum

import marshmallow_dataclass as md

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
    
JoinMessageSchema = md.class_schema(JoinMessage)()

@dataclass(kw_only=True)
class SdpMessage(SignalMessage):
    type: Literal['answer', 'offer', 'pranswer', 'rollback']
    sdp: str
    msgId: int
    
SdpMessageSchema = md.class_schema(SdpMessage)()
    
@dataclass(kw_only=True)
class SubscribeAddMessage(SignalMessage):
    op: Literal[SubOp.ADD] = SubOp.ADD
    reqTypes: list[str] | None = None
    pattern: Pattern
    
SubscribeAddMessageSchema = md.class_schema(SubscribeAddMessage)()
    
@dataclass(kw_only=True)
class SubscribeResultMessage(SignalMessage):
    id: str
    
SubscribeResultMessageSchema = md.class_schema(SubscribeResultMessage)()
    
@dataclass(kw_only=True)
class SubscribedMessage(SignalMessage):
    subId: str
    pubId: str
    sdpId: int
    tracks: list[Track]
    
SubscribedMessageSchema = md.class_schema(SubscribedMessage)()