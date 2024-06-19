import { Labels, Pattern } from './pattern';

export interface MessageRouter {
    room?: string;
    userFrom?: string;
    userTo?: string;
}

export interface SignalMessage {
    router?: MessageRouter
}

export interface SdpMessage extends SignalMessage {
    type: RTCSdpType;
    sdp: string;
    mid: number;
}

export interface CandidateMessage extends SignalMessage {
    op: "add" | "end";
    candidate?: RTCIceCandidateInit;
}

export interface CallFrame {
    filename: string;
    line: number;
    funcname: string;
}

export interface ErrorMessage extends SignalMessage {
    msg: string;
    cause: string;
    fatal: boolean;
    callFrames?: CallFrame[]
}

export interface PingMessage extends SignalMessage {
    msgId: number;
}

export interface PongMessage extends SignalMessage {
    msgId: number;
}

export interface ParticipantJoinMessage extends SignalMessage {
    userId: string;
    userName: string;
    socketId: string;
    joinId: number;
}

export interface ParticipantLeaveMessage extends SignalMessage {
    userId: string;
    socketId: string;
    joinId: number;
}

export interface Track {
    type: string;
    pubId: string;
    globalId: string;
    bindId: string;
    rid: string;
    streamId: string;
    labels?: Labels;
}

export interface JoinMessage extends SignalMessage {
    rooms?: string[]
}

export interface LeaveMessage extends SignalMessage {
    rooms?: string[]
}

export const PUB_OP_ADD = 0;
export const PUB_OP_REMOVE = 1;

export interface TrackToPublish {
    type: string;
    bindId: string;
    rid?: string;
    sid: string;
    labels?: Labels;
}

export interface PublishAddMessage extends SignalMessage {
    op: typeof PUB_OP_ADD;
    tracks: TrackToPublish[];
}

export interface PublishRemoveMessage extends SignalMessage {
    op: typeof PUB_OP_REMOVE;
    id: string;
}

export interface PublishResultMessage extends SignalMessage {
    id: string;
}

export interface PublishedMessage extends SignalMessage {
    track: Track;
}

export const SUB_OP_ADD = 0;
export const SUB_OP_UPDATE = 1;
export const SUB_OP_REMOVE = 2;

export interface SubscribeAddMessage extends SignalMessage {
    op: typeof SUB_OP_ADD;
    reqTypes?: string[];
    pattern: Pattern;
}

export interface SubscribeUpdateMessage extends SignalMessage {
    op: typeof SUB_OP_UPDATE;
    id: string;
    reqTypes?: string[];
    pattern: Pattern;
}

export interface SubscribeRemoveMessage extends SignalMessage {
    op: typeof SUB_OP_REMOVE;
    id: string;
}

export interface SubscribeResultMessage extends SignalMessage {
    id: string;
}

export interface SubscribedMessage extends SignalMessage {
    subId: string;
    pubId: string;
    sdpId: number;
    tracks: Track[];
}

export interface CustomMessage extends SignalMessage {
    content: string;
    msgId: number;
    ack: boolean;
}

export interface CustomMessageWithEvt {
    evt: string;
    msg: CustomMessage;
}

export interface CustomAckMessage extends SignalMessage {
    msgId: number;
    content: string;
    err: boolean;
}

export interface UserInfo {
    key: string;
    userId: string;
    userName: string;
    role: string;
    rooms: string[];
}