import "webrtc-adapter";
import { io, Socket } from 'socket.io-client';
import { Mutex } from 'async-mutex'
import Emittery from 'emittery';
import { v4 as uuidv4 } from 'uuid';
import * as sdpTransform from 'sdp-transform';

import PATTERN, { Labels, Pattern } from './pattern';
import { ERR_PEER_CLOSED, ERR_PEER_FAILED, TimeOutError, ServerError, SocketCloseError } from './errors';
import {
    PUB_OP_ADD,
    PUB_OP_REMOVE,
    SUB_OP_ADD,
    SUB_OP_REMOVE,
    CandidateMessage,
    CustomAckMessage,
    CustomMessage,
    ErrorMessage,
    JoinMessage,
    LeaveMessage,
    MessageRouter,
    ParticipantJoinMessage,
    PublishAddMessage,
    PublishedMessage,
    PublishRemoveMessage,
    PublishResultMessage,
    SdpMessage,
    SubscribeAddMessage,
    SubscribedMessage,
    SubscribeRemoveMessage,
    SubscribeResultMessage,
    Track,
    TrackToPublish,
    CustomMessageWithEvt,
    UserInfo,
    ParticipantLeaveMessage,
    PingMessage,
    PongMessage,
} from './message';
import { getLogger } from './log';
import { combineAsyncIterable } from "./async";
import { Timeouter, TimeoutHandler, makeTimeoutPromise, stopTimeoutHandler, StopEmitEventMap, makeTimeoutEvent, TimeoutEmitEventMap, timeoutEmit } from './timeout';
import { NamedEvent } from './types';

export const PT = PATTERN;
export type { Track } from './message';
export { Stopper } from './timeout';
export { TimeOutError, ServerError, SocketCloseError } from './errors';

function splitUrl(url: string) {
    const spos = url.indexOf('://');
    let startPos = 0
    if (spos != -1) {
        startPos = spos + 3
    }
    const pos = url.indexOf('/', startPos);
    if (pos != -1) {
        return [url.substring(0, pos), url.substring(pos)]
    } else {
        return [url, '/']
    }
}

export enum KeepAliveMode {
    ACTIVE,
    PASSIVE,
}
export interface KeepAliveContext {
    timeoutNum: number;
    timeoutDurationMs: number;
}
export type KeepAliveCallback = (ctx: KeepAliveContext) => boolean;

export interface StreamConstraint {
    type?: string;
    codec?: {
        profileLevelId?: string;
    };
}

export interface LocalStream {
    stream: MediaStream;
    labels?: Labels;
    constraints?: StreamConstraint[];
}

export interface Participant {
    userId: string;
    userName: string;
    socketId: string;
    joinId: number;
}

interface LeavedInfo {
    joinId: number;
    timestamp: number;
}

interface LeavedInfoMap {
    [key: string]: LeavedInfo;
}

interface Participants {
    participants: Participant[];
    index: Map<string, number>;
    leaves: Map<string, LeavedInfo>;
}

function ptsList(pts: Participants) {
    return pts.participants;
}

function ptsClean(pts: Participants) {
    const now = Date.now();
    for (const [key, info] of pts.leaves) {
        if (now > info.timestamp + 60 * 1000) {
            pts.leaves.delete(key);
        }
	}
	return (!pts.participants || pts.participants.length === 0) && (!pts.leaves || pts.leaves.size === 0);
}

function ptsAdd(pts: Participants, participant: Participant): boolean {
    const key = `${participant.userId}|${participant.socketId}`;
    const leaveInfo = pts.leaves.get(key);
    if (leaveInfo) {
        if (participant.joinId > leaveInfo.joinId) {
            pts.leaves.delete(key);
        } else {
            return false;
        }
    }
    let success = false;
    const pos = pts.index.get(key);
    if (pos != undefined) {
        const pt = pts.participants[pos];
        if (participant.joinId > pt.joinId) {
            pts.participants[pos] = participant;
            success = true;
        }
    } else {
        pts.participants.push(participant);
        pts.index.set(key, pts.participants.length - 1);
        success = true;
    }
    return success;
}

function ptsRemove(pts: Participants, uid: string, sid: string, jid: number): Participant | undefined {
	const key = `${uid}|${sid}`;
    const leaveInfo = pts.leaves.get(key);
    if (leaveInfo) {
        if (jid > leaveInfo.joinId) {
            leaveInfo.joinId = jid;
            leaveInfo.timestamp = Date.now();
        } else {
            return undefined;
        }
    } else {
        pts.leaves.set(key, {
            joinId: jid,
            timestamp: Date.now(),
        });
    }
    const pos = pts.index.get(key);
    if (pos != undefined) {
        const pt = pts.participants[pos];
        if (jid >= pt.joinId) {
            pts.participants.splice(pos, 1);
            pts.index.delete(key);
            return pt;
        }
    }
	return undefined;
}

type Ack = (...args: any[]) => any;

export interface TrackEvent {
    tracks: Track[];
    add: Track[];
    remove: Track[];
}

export type OnTrack = (tracks: TrackEvent) => any;
export type OnClose = (reason: string) => any;

interface ListenEventMap {
    ready: (msg: UserInfo) => void;
    ping: (msg: PingMessage) => void;
    pong: (msg: PongMessage) => void;
    subscribed: (msg: SubscribedMessage) => void;
    published: (msg: PublishedMessage) => void;
    error: (msg: ErrorMessage) => void;
    sdp: (msg: SdpMessage) => void;
    candidate: (msg: CandidateMessage) => void;
    "participant-join": (msg: ParticipantJoinMessage) => void;
    "participant-leave": (msg: ParticipantLeaveMessage) => void;
    custom: (msg: CustomMessage) => void;
    'custom-ack': (msg: CustomAckMessage) => void;
}

interface EmitEventMap {
    ping: (msg: PingMessage) => void;
    pong: (msg: PongMessage) => void;
    join: (msg: JoinMessage, ack: (res: any) => void) => void;
    leave: (msg: LeaveMessage, ack: (res: any) => void) => void;
    subscribe: (msg: SubscribeAddMessage | SubscribeRemoveMessage, ack: (res: SubscribeResultMessage) => void) => void;
    publish: ((msg: PublishAddMessage | PublishRemoveMessage, ack: (res: PublishResultMessage) => void) => void) | ((msg: PublishAddMessage | PublishRemoveMessage) => void);
    sdp: (msg: SdpMessage) => void;
    candidate: (msg: CandidateMessage) => void;
    custom: (msg: CustomMessage) => void;
    'custom-ack': (msg: CustomAckMessage) => void;
}

interface SocketConnectState {
    connected: boolean;
    reason?: string;
}

interface EventData {
    connect: NamedEvent<'connect', SocketConnectState>;
    disconnect: NamedEvent<'disconnect', SocketConnectState>;
    connectState: NamedEvent<'connectState', RTCPeerConnectionState>;
    ready: NamedEvent<'ready', UserInfo>;
    ping: NamedEvent<'ping', PingMessage>;
    pong: NamedEvent<'pong', PongMessage>;
    participantJoin: NamedEvent<'participantJoin', ParticipantJoinMessage>;
    participantLeave: NamedEvent<'participantLeave', { msg: ParticipantLeaveMessage, participant: Participant }>;
    track: NamedEvent<'track', [MediaStreamTrack, readonly MediaStream[], RTCRtpTransceiver]>;
    subscribed: NamedEvent<'subscribed', SubscribedMessage>;
    published: NamedEvent<'published', PublishedMessage>;
    sdp: NamedEvent<'sdp', SdpMessage>;
    error: NamedEvent<'error', ErrorMessage>;
    customMsg: NamedEvent<'customMsg', CustomMessageWithEvt>;
    customAckMsg: NamedEvent<'customAckMsg', CustomAckMessage>;
}

export interface Configuration {
    signalUrl: string;
    token: string;
    polite?: boolean;
    rtcConfig?: RTCConfiguration;
    name?: string;
}

export class ConferenceClient {
    private config: Configuration;
    private socket: Socket<ListenEventMap, EmitEventMap>;
    private ignoreOffer: boolean;
    private pendingCandidates: CandidateMessage[];
    private peer: RTCPeerConnection;
    private onTrasksCallbacks: OnTrack[];
    private onCloseCallback?: OnClose;
    private emitter: Emittery<EventData>;
    private sdpMsgId: number;
    private customMsgId: number;
    private pingMsgId: number;
    private negMux: Mutex;
    private userInfo: UserInfo;
    private participantsMap: Map<string, Participants>;
    private room: string
    private _id: string;

    constructor(config: Configuration) {
        this.config = config;
        this.room = '';
        const {
            signalUrl,
            token,
        } = config;
        const [host, path] = splitUrl(signalUrl);
        this.emitter = new Emittery()
        this.ignoreOffer = false;
        this.pendingCandidates = [];
        this.onTrasksCallbacks = [];
        this.participantsMap = new Map<string, Participants>();
        this.sdpMsgId = 1;
        this.customMsgId = 1;
        this.pingMsgId = 1;
        this.negMux = new Mutex();
        this.peer = this.createPeer();
        this._id = uuidv4();
        this.socket = io(host, {
            auth: {
                token,
                id: this._id,
            },
            path,
            transports: ['websocket'],
            upgrade: false,
            autoConnect: false,
            reconnection: false,
            // reconnectionDelay: 500,
            rememberUpgrade: true,
        });
        this.socket.on('ready', (info: UserInfo) => {
            this.userInfo = info;
            if (!info) {
                this.logger().error(`invalid ready msg.`);
                this.socket.disconnect();
            }
            if (!info.rooms) {
                info.rooms = [];
            }
            if (info.rooms.length == 1) {
                this.room = info.rooms[0];
            }
            this.emitter.emit('ready', {
                name: 'ready',
                data: info,
            });
        })
        this.socket.on('connect', () => {
            this.emitter.emit('connect', {
                name: 'connect',
                data: { connected: true },
            });
        });
        this.socket.on('disconnect', (reason) => {
            this.emitter.emit('disconnect', {
                name: 'disconnect',
                data: { connected: false, reason },
            });
            this.logger().warn(`socket disconnected because ${reason}`);
            if (this.onCloseCallback) {
                this.onCloseCallback(reason);
            }
        })
        this.socket.on('error', (msg: ErrorMessage) => {
            this.emitter.emit('error', {
                name: 'error',
                data: msg,
            });
            this.logger().error(`Received${msg.fatal ? " fatal " : " "}error ${msg.msg} because of ${msg.cause}`);
        });
        this.socket.on('ping', (msg: PingMessage) => {
            const router = msg.router;
            if (!router) {
                this.logger().warn('Receive a ping msg without router.');
                return;
            }
            this.emitter.emit('ping', {
                name: 'ping',
                data: msg,
            });
            this.socket.emit('pong', {
                router: {
                    room: router.room,
                    userTo: router.userFrom,
                },
                msgId: msg.msgId,
            });
        });
        this.socket.on('pong', (msg: PongMessage) => {
            const router = msg.router;
            if (!router) {
                this.logger().warn('Receive a pong msg without router.');
                return;
            }
            this.emitter.emit('pong', {
                name: 'pong',
                data: msg,
            });
        });
        this.socket.on('subscribed', (msg: SubscribedMessage, ack?: Ack) => {
            this.ack(ack);
            this.emitter.emit('subscribed', {
                name: 'subscribed',
                data: msg,
            });
        });
        this.socket.on('published', (msg: PublishedMessage, ack?: Ack) => {
            this.ack(ack);
            this.logger().debug(`received published msg for pub ${msg.track.pubId} and track ${msg.track.globalId}`);
            this.emitter.emit('published', {
                name: 'published',
                data: msg,
            });
        });
        this.socket.on("sdp", (msg: SdpMessage, ack?: Ack) => {
            this.ack(ack);
            this.emitter.emit('sdp', {
                name: 'sdp',
                data: msg,
            });
        });
        this.socket.on("candidate", async (msg: CandidateMessage, ack?: Ack) => {
            this.ack(ack);
            if (!this.peer.remoteDescription) {
                this.pendingCandidates.push(msg);
                return
            }
            await this.addCandidate(this.peer, msg);
        });
        this.socket.on("participant-join", (msg: ParticipantJoinMessage, ack?: Ack) => {
            this.ack(ack);
            const room = msg.router?.room;
            if (!room) {
                return;
            }
            const participant = {
                userId: msg.userId,
                userName: msg.userName,
                socketId: msg.socketId,
                joinId: msg.joinId,
            };
            let participants = this.participantsMap.get(room);
            let added = false;
            if (participants) {
                added = ptsAdd(participants, participant);
            } else {
                participants = {
                    participants: [],
                    index: new Map<string, number>(),
                    leaves: new Map<string, LeavedInfo>(),
                };
                added = ptsAdd(participants, participant);
                this.participantsMap.set(room, participants);
            }
            if (added) {
                this.emitter.emit('participantJoin', {
                    name: 'participantJoin',
                    data: msg,
                });
            }
        });
        this.socket.on("participant-leave", (msg: ParticipantLeaveMessage, ack?: Ack) => {
            this.ack(ack);
            const room = msg.router?.room;
            if (!room) {
                return;
            }
            let participants = this.participantsMap.get(room);
            let participant: Participant | undefined = undefined;
            if (participants) {
                participant = ptsRemove(participants, msg.userId, msg.socketId, msg.joinId);
            } else {
                participants = {
                    participants: [],
                    index: new Map<string, number>(),
                    leaves: new Map<string, LeavedInfo>(),
                };
                participant = ptsRemove(participants, msg.userId, msg.socketId, msg.joinId);
                this.participantsMap.set(room, participants);
            }
            for (const [room, pts] of this.participantsMap) {
                if (ptsClean(pts)) {
                    this.participantsMap.delete(room);
                }
            }
            if (participant) {
                this.emitter.emit('participantLeave', {
                    name: 'participantLeave',
                    data: {
                        msg,
                        participant,
                    },
                });
            }
        });
        this.socket.onAny((evt: string, msg: CustomMessage) => {
            if (evt.startsWith("custom:")) {
                evt = evt.substring(7);
                if (evt) {
                    this.emitter.emit('customMsg', {
                        name: 'customMsg',
                        data: {
                            evt,
                            msg,
                        },
                    });
                }
            }
        });
        this.socket.on('custom-ack', (msg: CustomAckMessage) => {
            this.emitter.emit('customAckMsg', {
                name: 'customAckMsg',
                data: msg,
            });
        });
    }

    id = () => {
        const _name = this.name();
        if (_name && this._id) {
            return `${_name}-${this._id}`;
        } else if (_name) {
            return _name;
        } else if (this._id) {
            return this._id;
        } else {
            return '';
        }
    }

    name = () => {
        return this.config.name || '';
    }

    logger = () => {
        return getLogger(`conference-${this.id()}`);
    }

    private nextSdpMsgId = () => {
        this.sdpMsgId += 2;
        return this.sdpMsgId;
    }

    private nextCustomMsgId = () => {
        return this.customMsgId ++;
    }

    private nextPingMsgId = () => {
        return this.pingMsgId ++;
    }

    private ack = (func?: Ack) => {
        if (func) {
            this.logger().debug("ack");
            func("ack");
        } else {
            this.logger().debug("no ack");
        }
    }

    private createPeer = () => {
        const peer = new RTCPeerConnection(this.config.rtcConfig);
        peer.onicecandidate = (evt) => {
            let msg: CandidateMessage;
            if (evt.candidate) {
                msg = {
                    op: "add",
                    candidate: evt.candidate.toJSON(),
                };
            } else {
                msg = {
                    op: "end",
                };
            }
            if (evt.candidate) {
                this.logger().log(`find and send candidate ${evt.candidate.address}`);
            } else {
                this.logger().log(`find and send the completed candidate`);
            }
            this.socket.emit("candidate", msg);
        };
        peer.onconnectionstatechange = () => {
            this.logger().log(`connection state changed to ${peer.connectionState}`);
            this.emitter.emit('connectState', {
                name: 'connectState',
                data: this.peer.connectionState,
            });
        };
        peer.oniceconnectionstatechange = () => {
            this.logger().log(`ice connection state changed to ${peer.iceConnectionState}`);
        };
        peer.onsignalingstatechange = () => {
            this.logger().log(`signaling state changed to ${peer.signalingState}`);
        };
        peer.onicegatheringstatechange = () => {
            this.logger().log(`ice gathering state changed to ${peer.signalingState}`);
        };
        peer.ontrack = async (evt) => {
            evt.track.onmute = (evt0) => {
                this.logger().log(`The remote track ${evt.track.id}/${evt.transceiver.mid} is muted`);
            }
            evt.track.onunmute = (evt0) => {
                this.logger().log(`The remote track ${evt.track.id}/${evt.transceiver.mid} is unmuted`);
            }
            evt.track.onended = (evt0) => {
                this.logger().log(`The remote track ${evt.track.id}/${evt.transceiver.mid} is ended`);
            }
            this.emitter.emit("track", {
                name: 'track',
                data: [evt.track, evt.streams, evt.transceiver],
            });
            this.logger().log(`Received track ${evt.track.id} with stream id ${evt.streams[0].id}`)
        };
        return peer;
    }

    private makeSureSocket = async (timeout: number, stopEmitter?: Emittery<StopEmitEventMap>) => {
        if (this.socket.connected) {
            this.logger().debug('already connected.');
            return;
        }
        this.logger().info("start connect socket...");
        const disconnectEvt = this.emitter.once('disconnect');
        const readyEvt = this.emitter.once('ready').then(() => ({ data: { connected: true, reason: '' } }));
        const stopEvt = stopEmitter ? stopEmitter.once('stop').then(() => {
            this.logger().debug('Received stop msg, it means timeout.');
            throw new TimeOutError();
        }) : new Promise<NamedEvent<'', SocketConnectState>>(() => {});
        const timeoutEvt = makeTimeoutPromise<NamedEvent<'', SocketConnectState>>(timeout);
        this.socket.connect();
        const { data } = await Promise.race([
            disconnectEvt,
            readyEvt,
            stopEvt,
            timeoutEvt,
        ]);
        if (!data.connected) {
            this.logger().error(`Unable to make sure the socket connection, because ${data.reason}`);
            this.socket.disconnect();
            throw new Error(data.reason);
        } else {
            this.logger().info("socket connected.");
        }
    }

    private addCandidate = async (peer: RTCPeerConnection, msg: CandidateMessage) => {
        try {
            if (msg.op === "end") {
                this.logger().log(`Received candidate completed`);
                await peer.addIceCandidate();
            } else {
                this.logger().log(`Received candidate ${msg.candidate.candidate}`);
                await peer.addIceCandidate(msg.candidate);
            }
        } catch (err) {
            if (!this.ignoreOffer) {
                throw err;
            }
        }
    }

    private getMid = (peer: RTCPeerConnection, transceiver: RTCRtpTransceiver): string => {
        if (transceiver.mid) {
            return transceiver.mid;
        } else {
            const transceivers = peer.getTransceivers();
            const pos = transceivers.indexOf(transceiver);
            if (pos === -1) {
                throw new Error("This is impossible.");
            }
            return `pos:${pos}`;
        }
    }

    private findTransceiverByBindId = (bindId: string): RTCRtpTransceiver | null => {
        let pos = -1;
        if (bindId.startsWith('pos:')) {
            pos = Number.parseInt(bindId.substring(4));
        }
        const transceivers = this.peer.getTransceivers();
        if (pos != -1) {
            if (pos >= transceivers.length) {
                throw new Error(`invalid bind id: ${bindId}, the extracted pos out of bound`);
            }
            return transceivers[pos];
        } else {
            for (const t of transceivers) {
                if (t.mid === bindId) {
                    return t;
                }
            }
            return null;
        }
    }

    private socketWithTimeout = (timeout: number) => {
        if (timeout > 0) {
            return this.socket.timeout(timeout);
        } else if (timeout === 0) {
            throw new TimeOutError();
        } else {
            return this.socket;
        }
    }

    join = async (timeout: number = -1, ...rooms: string[]) => {
        const timeouter = new Timeouter(timeout);
        await this.makeSureSocket(timeouter.left());
        await this.socketWithTimeout(timeouter.left()).emitWithAck('join', {
            rooms,
        });
        const myRooms = this.userInfo.rooms;
        for (const room of rooms) {
            if (myRooms.indexOf(room) === -1) {
                myRooms.push(room);
            }
        }
        this.userInfo.rooms = myRooms;
    }

    leave = async (timeout: number = -1, ...rooms: string[]) => {
        const timeouter = new Timeouter(timeout);
        await this.makeSureSocket(timeouter.left());
        await this.socketWithTimeout(timeouter.left()).emitWithAck('leave', {
            rooms,
        });
        const myRooms = this.userInfo.rooms;
        for (const room of rooms) {
            const pos = myRooms.indexOf(room);
            if (pos !== -1) {
                myRooms.splice(pos, 1);
            }
        }
        if (this.room && rooms.indexOf(this.room) !== -1) {
            this.room = '';
        }
    }

    toRoom = async (room: string, timeout: number = -1) => {
        const timeouter = new Timeouter(timeout);
        await this.makeSureSocket(timeouter.left());
        const myRooms = this.userInfo.rooms;
        if (myRooms.indexOf(room) === -1) {
            throw new Error(`no right to switch to room ${room}.`);
        }
        this.room = room;
        return this;
    }

    getUserInfo = async (timeout: number = -1) => {
        const timeouter = new Timeouter(timeout);
        await this.makeSureSocket(timeouter.left());
        return this.userInfo;
    }

    private checkRoom = () => {
        const room = this.room;
        if (!room) {
            throw new Error("no room is selected");
        }
        return room;
    }

    sendCustomMessageWithAck = async (evt: string, msg: any, to?: string, timeout: number = -1) => {
        const timeouter = new Timeouter(timeout);
        await this.makeSureSocket(timeouter.left());
        const room = this.checkRoom();
        const router: MessageRouter = {
            room: room,
            userTo: to,
        };
        const evts = combineAsyncIterable([this.emitter.events(['customAckMsg', 'disconnect', 'error']), timeouter.stopEvt()]);
        const msgId = this.nextCustomMsgId();
        this.socket.emit(`custom:${evt}` as "custom", {
            msgId,
            content: JSON.stringify(msg),
            router,
            ack: true,
        });
        for await (const evt of evts) {
            if (evt.name === 'disconnect') {
                await evts.return();
                throw new SocketCloseError(evt.data.reason);
            } else if (evt.name === 'error') {
                await evts.return();
                throw new ServerError(evt.data);
            } else if (evt.name === 'stop') {
                await evts.return();
                throw new TimeOutError();
            } else if (msgId === evt.data.msgId) {
                const router = evt.data.router;
                if (router?.room === room && router?.userFrom === to) {
                    await evts.return();
                    const content = JSON.parse(evt.data.content);
                    if (evt.data.err) {
                        throw new ServerError(content as ErrorMessage);
                    } else {
                        return content;
                    }
                }
            }
        }
    };

    sendCustomMessage = async (evt: string, msg: any, to?: string, timeout: number = -1) => {
        const timeouter = new Timeouter(timeout);
        await this.makeSureSocket(timeouter.left());
        const room = this.checkRoom();
        const router: MessageRouter = {
            room: room,
            userTo: to,
        };
        const msgId = this.nextCustomMsgId();
        this.socket.emit(`custom:${evt}` as "custom", {
            msgId,
            content: JSON.stringify(msg),
            router,
            ack: false,
        });
    };

    waitCustomMessage = async (checker: (evt:string, from?: string, to?: string) => boolean, timeout: number = -1): Promise<{ resp: any, ack?: (res: any, err: ErrorMessage) => void}> => {
        const timeouter = new Timeouter(timeout);
        await this.makeSureSocket(timeouter.left());
        const room = this.checkRoom();
        const evts = combineAsyncIterable([this.emitter.events(['customMsg', 'disconnect', 'error']), timeouter.stopEvt()]);
        for await (const evt of evts) {
            if (evt.name === 'disconnect') {
                await evts.return();
                throw new SocketCloseError(evt.data.reason);
            } else if (evt.name === 'error') {
                await evts.return();
                throw new ServerError(evt.data);
            } else if (evt.name === 'stop') {
                await evts.return();
                throw new TimeOutError();
            } else {
                const msg_with_evt = evt.data;
                const msg = msg_with_evt.msg;
                const msg_evt = msg_with_evt.evt;
                if (msg_evt && msg) {
                    if (room === msg.router?.room && checker(msg_evt, msg.router?.userFrom, msg.router?.userTo)) {
                        await evts.return();
                        const content = msg.content ? JSON.parse(msg.content) : undefined;
                        if (msg.ack) {
                            return {
                                resp: content,
                                ack: (res: any, err: ErrorMessage) => {
                                    this.socket.emit('custom-ack', {
                                        router: {
                                            room: msg.router?.room,
                                            userTo: msg.router?.userFrom,
                                        },
                                        msgId: msg.msgId,
                                        content: JSON.stringify(err ? err: res),
                                        err: !!err,
                                    });
                                },
                            }
                        }
                        return content;
                    }
                }
            }
        }
    };

    waitParticipant = async (uid: string, timeout: number = -1, stopEmitter?: Emittery<StopEmitEventMap>): Promise<boolean> => {
        const room = this.checkRoom();
        const tgtEvts = this.emitter.events(['participantJoin', 'disconnect', 'error']);
        const emitter = new Emittery<TimeoutEmitEventMap>();
        const [timeoutEvts, clearTimeoutEvt] = makeTimeoutEvent(emitter, timeout);
        const evts = combineAsyncIterable([tgtEvts, timeoutEvts, stopEmitter.events('stop')]);
        const participants = this.participantsMap.get(room);
        if (participants && participants.participants.some((p) => p.userId === uid)) {
            return true;
        }
        for await (const evt of evts) {
            switch (evt.name) {
                case 'participantJoin':
                    clearTimeoutEvt();
                    await evts.return();
                    return true;
                case "disconnect":
                    clearTimeoutEvt();
                    await evts.return();
                    throw new SocketCloseError(evt.data.reason);
                case "error":
                    clearTimeoutEvt();
                    await evts.return();
                    throw new ServerError(evt.data);
                case "timeout":
                    clearTimeoutEvt();
                    await evts.return();
                    throw new TimeOutError();
                case "stop":
                    clearTimeoutEvt();
                    await evts.return();
                    return false;
            }
        }
    }

    keepAlive = async (uid: string, mode: KeepAliveMode, cb?: KeepAliveCallback, timeout: number = -1, stopEmitter?: Emittery<StopEmitEventMap>) => {
        await this.makeSureSocket(timeout, stopEmitter);
        const room = this.checkRoom();
        if (!cb) {
            cb = (ctx) => {
                return ctx.timeoutNum > 3;
            }
        }
        if (timeout <= 0) {
            timeout = 3000;
        }
        const keepAlive = async () => {
            const ctx: KeepAliveContext = {
                timeoutNum: 0,
                timeoutDurationMs: 0,
            };
            let err: Error;
            if (mode === KeepAliveMode.ACTIVE) {
                const emitter = new Emittery<TimeoutEmitEventMap>();
                while (true) {
                    const now = new Date().getTime();
                    const msgId = this.nextPingMsgId();
                    const [timeoutEvts, clearTimeoutEvt] = makeTimeoutEvent(emitter, timeout);
                    const tgtEvts = this.emitter.events(['pong', 'disconnect', 'error']);
                    const evts = combineAsyncIterable([
                        tgtEvts,
                        stopEmitter.events('stop'),
                        timeoutEvts,
                    ])
                    this.socket.emit('ping', {
                        router: {
                            userTo: uid,
                            room,
                        },
                        msgId,
                    });
                    let err: Error;
                    for await (const evt of evts) {
                        switch(evt.name) {
                            case 'stop':
                                clearTimeoutEvt();
                                await evts.return();
                                return;
                            case "disconnect":
                                clearTimeoutEvt();
                                await evts.return();
                                throw new SocketCloseError(evt.data.reason);
                            case "error":
                                clearTimeoutEvt();
                                await evts.return();
                                throw new ServerError(evt.data);
                            case "timeout":
                                clearTimeoutEvt();
                                await evts.return();
                                ctx.timeoutNum ++;
                                ctx.timeoutDurationMs += new Date().getTime() - now;
                                if (cb(ctx)) {
                                    throw new TimeOutError();
                                } else {
                                    break;
                                }
                            case "pong":
                                const router = evt.data.router;
                                if (router.room == room && router.userFrom == uid && evt.data.msgId === msgId) {
                                    clearTimeoutEvt();
                                    await evts.return();
                                    ctx.timeoutNum = 0;
                                    ctx.timeoutDurationMs = 0;
                                }
                                break;
                        }
                    }
                }
            } else {
                const emitter = new Emittery<TimeoutEmitEventMap>();
                const timeoutEvts = emitter.events('timeout');
                const tgtEvts = this.emitter.events(['ping', 'disconnect', 'error']);
                const evts = combineAsyncIterable([
                    tgtEvts,
                    stopEmitter.events('stop'),
                    timeoutEvts,
                ])
                let now = new Date().getTime();
                let timeoutClear = timeoutEmit(timeoutEvts, emitter, timeout);
                for await (const evt of evts) {
                    switch(evt.name) {
                        case 'stop':
                            timeoutClear();
                            await evts.return();
                            return;
                        case 'disconnect':
                            timeoutClear();
                            await evts.return();
                            throw new SocketCloseError(evt.data.reason);
                        case 'error':
                            timeoutClear();
                            await evts.return();
                            throw new ServerError(evt.data);
                        case 'timeout':
                            ctx.timeoutNum ++;
                            ctx.timeoutDurationMs += new Date().getTime() - now;
                            now = new Date().getTime();
                            if (cb(ctx)) {
                                await evts.return();
                                return;
                            } else {
                                break;
                            }
                        case 'ping':
                            ctx.timeoutNum = 0;
                            ctx.timeoutDurationMs = 0;
                            now = new Date().getTime();
                            timeoutClear();
                            timeoutClear = timeoutEmit(timeoutEvts, emitter, timeout);
                            break;
                    }
                }
            }
        };
        await keepAlive();
    }

    private waitForEvt = async <E extends keyof EventData>(event: E, checker: (evt: EventData[E]) => boolean, initer: () => boolean) => {
        if (initer()) {
            return
        }
        const evts = this.emitter.events(event);
        for await (const evt of evts) {
            if (checker(evt)) {
                await evts.return();
                break;
            }
        }
    }

    private waitForNotConnecting = async () => {
        return await this.waitForEvt(
            'connectState',
            (evt) => evt.data !== 'connecting',
            () => this.peer.connectionState !== 'connecting',
        );
    };

    private applyStreamConstraint = (sdp: sdpTransform.SessionDescription, constraint: StreamConstraint) => {
        if (constraint.codec?.profileLevelId) {
            if (sdp.media) {
                for (const m of sdp.media) {
                    if (constraint.type && constraint.type !== m.type) {
                        continue;
                    }
                    let pts: number[] = [];
                    if (m.fmtp) {
                        for (const fmtp of m.fmtp) {
                            const config = sdpTransform.parseParams(fmtp.config);
                            if (config && 'profile-level-id' in config 
                                && typeof config['profile-level-id'] === 'string' 
                                && config['profile-level-id'].toLowerCase() === constraint.codec?.profileLevelId?.toLowerCase()) {
                                pts.push(fmtp.payload);
                            }
                            // if (config && 'level-asymmetry-allowed' in config
                            //     && typeof config['level-asymmetry-allowed'] === 'number'
                            //     && config['level-asymmetry-allowed'] === 1) {
                            //     config['level-asymmetry-allowed'] = 0;
                            //     fmtp.config = Object.keys(config).map(key => `${key}=${config[key]}`).join(';');
                            // }
                        }
                        for (const fmtp of m.fmtp) {
                            const config = sdpTransform.parseParams(fmtp.config);
                            if (config && 'apt' in config) {
                                const pt = config['apt'];
                                if (typeof pt === 'number' && pts.indexOf(pt) >= 0) {
                                    pts.push(fmtp.payload);
                                }
                            }
                        }
                        const oldPts = sdpTransform.parsePayloads(m.payloads);
                        const otherPts = oldPts.filter(pt => pts.indexOf(pt) < 0);
                        pts = pts.sort((a, b) => a - b);
                        for (const pt of otherPts) {
                            pts.push(pt);
                        }
                        m.payloads = pts.join(' ');
                    }
                }
            }
        }
    };

    private negotiate = async (
        sdpId: number, 
        active: boolean, 
        sdpEvts: AsyncIterableIterator<NamedEvent<'sdp', SdpMessage>> | null = null, 
        stopEvts: AsyncIterableIterator<
            NamedEvent<'stop', undefined> | NamedEvent<'disconnect', SocketConnectState> | NamedEvent<'error', ErrorMessage>
        > | null = null, 
        localStreamConstraints: StreamConstraint[] = []
    ) => {
        const peer = this.peer;
        await this.waitForNotConnecting();
        if (peer.connectionState === 'closed') {
            throw ERR_PEER_CLOSED;
        }
        if (active) {
            if (sdpEvts !== null) {
                throw new Error(`sdpEvts should be null when active is true.`);
            }
            let offer = await peer.createOffer();
            const offerObj = sdpTransform.parse(offer.sdp);
            for (const constraints of localStreamConstraints) {
                this.applyStreamConstraint(offerObj, constraints);
            }
            offer.sdp = sdpTransform.write(offerObj);
            await peer.setLocalDescription(offer);
            const desc = peer.localDescription;
            this.logger().debug(`create offer and set local desc`);
            const localSdp = sdpTransform.parse(desc.sdp);
            console.log(localSdp);

            sdpEvts = this.emitter.events('sdp');
            const evts = combineAsyncIterable([sdpEvts, stopEvts]);
            this.socket.emit('sdp', {
                type: desc.type,
                sdp: desc.sdp,
                mid: sdpId,
            });
            while (true) {
                let sdpMsg: SdpMessage;
                for await (const evt of evts) {
                    if (evt.name === 'sdp') {
                        if (evt.data.mid === sdpId) {
                            if (evt.data.type === 'answer' || evt.data.type === 'pranswer') {
                                this.logger().debug(`receive remote ${evt.data.type} sdp`);
                                sdpMsg = evt.data;
                                break;
                            } else {
                                throw new Error(`Expect an answer or pranswer, but got ${evt.data.type}. The sdp:\n${evt.data.sdp}`);
                            }
                        }
                    } else if (evt.name === 'stop') {
                        evts.return();
                        throw new TimeOutError();
                    } else if (evt.name === 'disconnect') {
                        evts.return();
                        throw new SocketCloseError(evt.data.reason);
                    } else if (evt.name === 'error' && evt.data.fatal) {
                        evts.return();
                        throw new ServerError(evt.data);
                    }
                }
                this.logger().debug('set remote desc');
                const remoteSdp = sdpTransform.parse(sdpMsg.sdp);
                console.log(remoteSdp);
                await peer.setRemoteDescription({
                    type: sdpMsg.type,
                    sdp: sdpMsg.sdp,
                });
                this.logger().debug('remote desc has set')
                for (const pending of this.pendingCandidates) {
                    await this.addCandidate(peer, pending);
                }
                this.pendingCandidates = [];
                if (sdpMsg.type === 'answer') {
                    await evts.return();
                    this.logger().debug('the remote desc is answer, so break out');
                    break;
                }
            }
        } else {
            if (sdpEvts === null) {
                throw new Error(`sdpEvts should not be null when active is false.`);
            }
            const evts = combineAsyncIterable([sdpEvts, stopEvts]);
            let sdpMsg: SdpMessage;
            for await (const evt of evts) {
                if (evt.name === 'sdp') {
                    if (evt.data.mid === sdpId) {
                        if (evt.data.type === 'offer') {
                            this.logger().debug(`receive remote ${evt.data.type} sdp`);
                            sdpMsg = evt.data;
                            await evts.return();
                            break;
                        } else {
                            throw new Error(`Expect an offer, but got ${evt.data.type}. The sdp:\n${evt.data.sdp}`);
                        }
                    }
                } else if (evt.name === 'stop') {
                    await evts.return();
                    throw new TimeOutError();
                } else if (evt.name === 'disconnect') {
                    evts.return();
                    throw new SocketCloseError(evt.data.reason);
                } else if (evt.name === 'error' && evt.data.fatal) {
                    evts.return();
                    throw new ServerError(evt.data);
                }
            }
            this.logger().debug('set remote desc');
            await peer.setRemoteDescription({
                type: sdpMsg.type,
                sdp: sdpMsg.sdp,
            });
            this.logger().debug('remote desc has set')
            for (const pending of this.pendingCandidates) {
                await this.addCandidate(peer, pending);
            }
            this.pendingCandidates = [];
            this.logger().debug('create answer');
            const answer = await peer.createAnswer();
            this.logger().debug('send answer');
            this.socket.emit('sdp', {
                type: 'answer',
                sdp: answer.sdp,
                mid: sdpId,
            });
            this.logger().debug('set local desc');
            await peer.setLocalDescription(answer);
            this.logger().debug('local desc has set');
        }
        // await this.waitForStableConnectionState()
        // if (this.peer.connectionState == 'closed') {
        //     throw ERR_PEER_CLOSED;
        // } else if (this.peer.connectionState == 'failed') {
        //     throw ERR_PEER_FAILED;
        // }
    }

    publish = async (stream: LocalStream, timeout: number = -1) => {
        const timeouter = new Timeouter(timeout);
        const cleaner = {
            stop: false,
            stopEmitter: new Emittery<StopEmitEventMap>(),
        };
        const timeoutHandler: TimeoutHandler = {};
        if (this.negMux.isLocked()) {
            this.logger().debug('There is another publish or subscribe task running, wait it finished.');
        }
        const task = this.negMux.runExclusive(async () => {
            await this.makeSureSocket(timeouter.left(), cleaner.stopEmitter);
            const peer = this.peer;
            this.logger().debug('start publish');
            const tracks: TrackToPublish[] = [];
            const senders: RTCRtpSender[] = [];
            for (const track of stream.stream.getTracks()) {
                const transceiver = peer.addTransceiver(track, {
                    direction: 'sendrecv',
                    streams: [stream.stream],
                });
                const t = {
                    type: track.kind,
                    bindId: this.getMid(peer, transceiver),
                    sid: stream.stream.id,
                    labels: stream.labels,
                };
                tracks.push(t);
                senders.push(transceiver.sender);
                this.logger().debug(`add track ${JSON.stringify(t)}`);
            }
            if (tracks.length === 0) {
                return "";
            }
            const cleanTracks = () => {
                stopTimeoutHandler(timeoutHandler);
                for (const sender of senders) {
                    if (sender) {
                        peer.removeTrack(sender);
                    }
                }
            };
            if (cleaner.stop) {
                cleanTracks();
                return "";
            }
            this.logger().debug('send publish msg');
            // publish must happen before negotiate, because in server, bind only happen in onTrack, which must ensure publication exists
            let pubId: string = '';
            try {
                const { id } = await this.socketWithTimeout(timeouter.left()).emitWithAck('publish', {
                    op: PUB_OP_ADD,
                    tracks,
                });
                pubId = id;
            } catch (e) {
                throw new TimeOutError();
            }
            const cleanAll = async () => {
                cleanTracks();
                await this._unpublish(pubId, false, -1);
            }
            this.logger().debug(`accept publish id ${pubId}`);
            const sdpId = this.nextSdpMsgId();
            this.logger().debug(`gen sdp id ${sdpId}`);
            let stopEvt = cleaner.stopEmitter.events('stop');
            if (cleaner.stop) {
                await cleanAll();
                return "";  
            }
            const evts = combineAsyncIterable([this.emitter.events(['published', 'connectState', 'disconnect', 'error']), stopEvt]);
            try {
                if (cleaner.stop) {
                    await cleanAll();
                    return "";  
                }
                await this.negotiate(
                    sdpId, true, null, 
                    combineAsyncIterable([this.emitter.events(['disconnect', 'error']), cleaner.stopEmitter.events('stop')]), 
                    stream.constraints
                );
            } catch (e) {
                await cleanAll();
                throw e;
            }
            let pubNum = 0;
            for await (const evt of evts) {
                if (evt.name === 'connectState') {
                    if (evt.data === 'closed') {
                        this.logger().debug('peer is closed');
                        await evts.return();
                        await cleanAll();
                        throw ERR_PEER_CLOSED;
                    } else if (evt.data === 'failed') {
                        this.logger().debug('peer is failed');
                        await evts.return();
                        await cleanAll();
                        throw ERR_PEER_FAILED;
                    }
                } else if (evt.name === 'stop') {
                    await evts.return();
                    await cleanAll();
                    this.logger().debug('receive timeout msg, so return.');
                    return ""
                } else if (evt.name === 'disconnect') {
                    await evts.return();
                    await cleanAll();
                    throw new SocketCloseError(evt.data.reason);
                } else if (evt.name === 'error') {
                    await evts.return();
                    await cleanAll();
                    throw new ServerError(evt.data);
                } else {
                    if (evt.data.track.pubId === pubId) {
                        const t = this.findTransceiverByBindId(evt.data.track.bindId);
                        if (t === null) {
                            this.logger().error('receive a unknown published track');
                            await evts.return();
                            await cleanAll();
                            throw Error('receive a unknown published track');
                        }
                        if (!t.sender.track) {
                            this.logger().error('receive a invalid published track');
                            await evts.return();
                            await cleanAll();
                            throw Error('receive a invalid published track');
                        }
                        this.logger().debug(`track ${t.sender.track.id} is published`);
                        pubNum++;
                        if (pubNum === tracks.length) {
                            await evts.return();
                            break;
                        }
                    } else {
                        this.logger().debug('receive a unknown published track');
                    }
                }
            }
            if (cleaner.stop) {
                await cleanAll();
                return "";
            }
            this.logger().debug(`publish ${pubId} completed`);
            stopTimeoutHandler(timeoutHandler);
            return pubId;
        });
        if (timeout > 0) {
            const resHandler = {
                pubId: '',
            };
            try {
                const res = await Promise.race([
                    task.then(id => {
                        resHandler.pubId = id;
                        return id;
                    }),
                    makeTimeoutPromise<string>(timeouter.left(), timeoutHandler),
                ])
                return res;
            } catch (e) {
                if (e instanceof TimeOutError) {
                    cleaner.stop = true;
                    cleaner.stopEmitter.emit("stop", {
                        name: 'stop',
                        data: undefined,
                    });
                    this.logger().debug('send stop msg.');
                    if (resHandler.pubId) {
                        await this._unpublish(resHandler.pubId, false, -1);
                    }
                }
                throw e;
            }
        } else {
            return task;
        }
    };

    private _unpublish = async (pubId: string, ack: boolean, timeout: number) => {
        if (ack) {
            const timeouter = new Timeouter(timeout);
            const res = await this.socketWithTimeout(timeouter.left()).emitWithAck('publish', {
                op: PUB_OP_REMOVE,
                id: pubId,
            });
            return res.id !== "";
        } else {
            this.socket.emit('publish', {
                op: PUB_OP_REMOVE,
                id: pubId,
            });
            return true;
        }
    };

    unpublish = async (pubId: string, timeout: number = -1) => {
        return this.negMux.runExclusive(() => {
            return this._unpublish(pubId, true, timeout);
        });
    }

    private checkTrack = (track: Track, transceiver: RTCRtpTransceiver) => {
        if (track.bindId.startsWith("pos:")) {
            const transceivers = this.peer.getTransceivers();
            const pos = transceivers.indexOf(transceiver);
            if (pos === -1) {
                return false;
            } else {
                return track.bindId === `pos:${pos}`;
            }
        } else {
            return track.bindId === transceiver.mid;
        }
    }

    subscribe = async (pattern: Pattern, reqTypes: string[] = [], timeout: number = 0) => {
        const timeouter = new Timeouter(timeout);
        const cleaner = {
            stop: false,
            stopEmitter: new Emittery<StopEmitEventMap>(),
        };
        const timeoutHandler: TimeoutHandler = {};
        if (this.negMux.isLocked()) {
            this.logger().debug('There is another publish or subscribe task running, wait it finished.');
        }
        const task = this.negMux.runExclusive(async () => {
            await this.makeSureSocket(timeouter.left(), cleaner.stopEmitter);
            if (cleaner.stop) {
                return {
                    subId: "",
                    stream: undefined as MediaStream,
                };
            }
            this.logger().debug('start subscribe');
            const sdpEvts = this.emitter.events('sdp');
            let stopEvt = cleaner.stopEmitter.events('stop');
            const subEvts = combineAsyncIterable([this.emitter.events(['subscribed', 'error', 'disconnect']), stopEvt]);
            stopEvt = cleaner.stopEmitter.events('stop');
            const trackOrStateEvts = combineAsyncIterable([this.emitter.events(['track', 'connectState', 'error', 'disconnect']), stopEvt]);
            this.logger().debug('send sub msg');
            let subId: string = '';
            try {
                const { id } = await this.socketWithTimeout(timeouter.left()).emitWithAck('subscribe', {
                    op: SUB_OP_ADD,
                    reqTypes,
                    pattern,
                });
                subId = id;
            } catch (e) {
                throw new TimeOutError();
            }
            this.logger().debug(`accept sub msg ark with sub id ${subId}`)
            const clean = async () => {
                stopTimeoutHandler(timeoutHandler);
                await this._unsubscribe(subId);
            }
            if (cleaner.stop) {
                await clean();
                return {
                    subId: "",
                    stream: undefined as MediaStream,
                };
            }
            let subedMsg: SubscribedMessage;
            for await (const subEvt of subEvts) {
                if (subEvt.name === 'subscribed') {
                    if (subEvt.data.subId === subId) {
                        subedMsg = subEvt.data;
                        subEvts.return();
                        break;
                    }
                } else if (subEvt.name === 'stop') {
                    this.logger().debug('receive timeout msg, so return.');
                    subEvts.return();
                    await clean();
                    return {
                        subId: "",
                        stream: undefined as MediaStream,
                    };
                } else if (subEvt.name === 'disconnect') {
                    subEvts.return();
                    await clean();
                    throw new SocketCloseError(subEvt.data.reason);
                } else {
                    if (subEvt.data.fatal) {
                        subEvts.return();
                        await clean();
                        throw new ServerError(subEvt.data);
                    }
                }
            }
            const { tracks, sdpId } = subedMsg;
            this.logger().debug(`accept subed msg with sub id ${subedMsg.subId}`)
            try {
                if (cleaner.stop) {
                    await clean();
                    return {
                        subId: "",
                        stream: undefined as MediaStream,
                    };
                }
                await this.negotiate(sdpId, false, sdpEvts, combineAsyncIterable([
                    this.emitter.events(['error', 'disconnect']), 
                    cleaner.stopEmitter.events('stop')
                ]));
            } catch (e) {
                await clean();
                throw e;
            }
            const resolved: Track[] = [];
            let stream: MediaStream;
            for await (const evt of trackOrStateEvts) {
                if (evt.name === 'connectState') {
                    if (evt.data === 'closed') {
                        trackOrStateEvts.return();
                        throw ERR_PEER_CLOSED;
                    } else if (evt.data === 'failed') {
                        trackOrStateEvts.return();
                        throw ERR_PEER_FAILED;
                    }
                } else if (evt.name === 'track') {
                    const [_, streams, transceiver] = evt.data;
                    for (const t of tracks) {
                        if (this.checkTrack(t, transceiver)) {
                            stream = streams[0]
                            resolved.push(t)
                            break
                        }
                    }
                } else if (evt.name === 'stop') {
                    this.logger().debug('receive timeout msg, so return.');
                    trackOrStateEvts.return();
                    await clean();
                    return {
                        subId: "",
                        stream: undefined as MediaStream,
                    };
                } else if (evt.name === 'disconnect') {
                    subEvts.return();
                    await clean();
                    throw new SocketCloseError(evt.data.reason);
                } else {
                    if (evt.data.fatal) {
                        trackOrStateEvts.return();
                        throw new ServerError(evt.data);
                    }
                }
                if (resolved.length === tracks.length) {
                    trackOrStateEvts.return();
                    break
                }
            }
            if (cleaner.stop) {
                await clean();
                return {
                    subId: "",
                    stream: undefined as MediaStream,
                };
            }
            this.logger().debug(`subscribe completed`)
            stopTimeoutHandler(timeoutHandler);
            return {
                subId: subId,
                stream,
            };
        });
        if (timeout > 0) {
            const resHandler = {
                subId: '',
            };
            try {
                const res = await Promise.race([
                    task.then(res => {
                        resHandler.subId = res.subId;
                        return res;
                    }),
                    makeTimeoutPromise<{ subId: string, stream: MediaStream }>(timeouter.left(), timeoutHandler),
                ]);
                return res;
            } catch (e) {
                if (e instanceof TimeOutError) {
                    cleaner.stop = true;
                    cleaner.stopEmitter.emit("stop", {
                        name: 'stop',
                        data: undefined,
                    });
                    this.logger().debug('send stop msg.');
                    if (resHandler.subId) {
                        await this._unsubscribe(resHandler.subId);
                    }
                }
                throw e;
            }
        } else {
            return task;
        }
    }

    private _unsubscribe = async (subId: string) => {
        const res = await this.socket.emitWithAck('subscribe', {
            op: SUB_OP_REMOVE,
            id: subId,
        });
        return res.id !== "";
    }

    unsubscribe = async (subId: string) => {
        return this.negMux.runExclusive(() => {
            return this._unsubscribe(subId);
        });
    }

    onTracks = (callback: OnTrack) => {
        this.onTrasksCallbacks.push(callback);
    }

    offTracks = (callback: OnTrack) => {
        const i = this.onTrasksCallbacks.indexOf(callback);
        if (i != -1) {
            this.onTrasksCallbacks.splice(i, 1);
        }
    }

    setOnClose = (callback: OnClose) => {
        this.onCloseCallback = callback;
    }
}