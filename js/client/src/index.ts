import "webrtc-adapter";
import { io, Socket } from 'socket.io-client';
import { Mutex } from 'async-mutex'
import Emittery from 'emittery';
import { v4 as uuidv4 } from 'uuid';

import PATTERN, { Labels, Pattern } from './pattern';
import { ERR_PEER_CLOSED, ERR_PEER_FAILED } from './errors';
import { getLogger } from './log';
import { combineAsyncIterable } from "./async";

export const PT = PATTERN;

export class TimeOutError extends Error {
    cleaned: boolean

    constructor() {
        super();
        Object.setPrototypeOf(this, TimeOutError.prototype);
        this.cleaned = false;
    }
}

export class ServerError extends Error {

    data: ErrorMessage

    constructor(msg: ErrorMessage) {
        super(msg.msg);
        Object.setPrototypeOf(this, ServerError.prototype);
        this.data = msg;
    }
}

async function withTimeout<T>(promise: Promise<T>, ms: number, cleaner: () => any = () => undefined): Promise<T> {
    try {
        return await Promise.race([promise, new Promise<T>((_, reject) => {
            setTimeout(() => {
                reject(new TimeOutError());
            }, ms);
        })])
    } catch(e) {
        if (e instanceof TimeOutError && !e.cleaned) {
            e.cleaned = true;
            if (cleaner) {
                cleaner();
            }
        }
        throw e;
    }
}

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

interface LocalStream {
    stream: MediaStream;
    labels?: Labels;
}

interface SignalMessage {
    to?: string
}

interface SdpMessage extends SignalMessage {
    type: RTCSdpType;
    sdp: string;
    mid: number;
}

interface CandidateMessage extends SignalMessage {
    op: "add" | "end";
    candidate?: RTCIceCandidateInit;
}

interface CallFrame {
    filename: string;
    line: number;
    funcname: string;
}

interface ErrorMessage extends SignalMessage {
    msg: string;
    cause: string;
    fatal: boolean;
    callFrames?: CallFrame[]
}

interface Track {
    type: string;
    pubId: string;
    globalId: string;
    bindId: string;
    rid: string;
    streamId: string;
    labels?: Labels;
}

interface JoinMessage extends SignalMessage {
    rooms?: string[]
}

interface LeaveMessage extends SignalMessage {
    rooms?: string[]
}

const PUB_OP_ADD = 0;
const PUB_OP_REMOVE = 1;

interface TrackToPublish {
    type: string;
    bindId: string;
    rid?: string;
    sid: string;
    labels?: Labels;
}

interface PublishAddMessage extends SignalMessage {
    op: typeof PUB_OP_ADD;
    tracks: TrackToPublish[];
}

interface PublishRemoveMessage extends SignalMessage {
    op: typeof PUB_OP_REMOVE;
    id: string;
}

interface PublishResultMessage extends SignalMessage {
    id: string;
}

interface PublishedMessage extends SignalMessage {
    track: Track;
}

const SUB_OP_ADD = 0;
const SUB_OP_UPDATE = 1;
const SUB_OP_REMOVE = 2;

interface SubscribeAddMessage extends SignalMessage {
    op: typeof SUB_OP_ADD;
    reqTypes?: string[];
    pattern: Pattern;
}

interface SubscribeUpdateMessage extends SignalMessage {
    op: typeof SUB_OP_UPDATE;
    id: string;
    reqTypes?: string[];
    pattern: Pattern;
}

interface SubscribeRemoveMessage extends SignalMessage {
    op: typeof SUB_OP_REMOVE;
    id: string;
}

interface SubscribeResultMessage extends SignalMessage {
    id: string;
}

interface SubscribedMessage extends SignalMessage {
    subId: string;
    pubId: string;
    sdpId: number;
    tracks: Track[];
}

type Ack = (...args: any[]) => any;

interface TrackEvent {
    tracks: Track[];
    add: Track[];
    remove: Track[];
}

type OnTrack = (tracks: TrackEvent) => any;

interface ListenEventMap {
    subscribed: (msg: SubscribedMessage) => void;
    published: (msg: PublishedMessage) => void;
    error: (msg: ErrorMessage) => void;
    sdp: (msg: SdpMessage) => void;
    candidate: (msg: CandidateMessage) => void;
    want: (msg: any) => void;
    state: (msg: any) => void;
    select: (msg: any) => void;
}

interface EmitEventMap {
    join: (msg: JoinMessage, ark: (res: any) => void) => void;
    leave: (msg: LeaveMessage, ark: (res: any) => void) => void;
    subscribe: (msg: SubscribeAddMessage | SubscribeRemoveMessage, ark: (res: SubscribeResultMessage) => void) => void;
    publish: (msg: PublishAddMessage | PublishRemoveMessage, ark: (res: PublishResultMessage) => void) => void;
    sdp: (msg: SdpMessage) => void;
    candidate: (msg: CandidateMessage) => void;
    want: (msg: any) => void;
    state: (msg: any) => void;
    select: (msg: any) => void;
}

interface StopEmitEventMap {
    stop: NamedEvent<'stop', undefined>;
}

interface SocketConnectState {
    connected: boolean;
    reason?: string;
}

interface NamedEvent<K, T> {
    name: K;
    data: T;
}

interface EventData {
    connect: NamedEvent<'connect', SocketConnectState>;
    disconnect: NamedEvent<'disconnect', SocketConnectState>;
    connectState: NamedEvent<'connectState', RTCPeerConnectionState>;
    track: NamedEvent<'track', [MediaStreamTrack, readonly MediaStream[], RTCRtpTransceiver]>;
    subscribed: NamedEvent<'subscribed', SubscribedMessage>;
    published: NamedEvent<'published', PublishedMessage>;
    sdp: NamedEvent<'sdp', SdpMessage>;
    error: NamedEvent<'error', ErrorMessage>;
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
    private emitter: Emittery<EventData>;
    private sdpMsgId: number;
    private negMux: Mutex;
    private _id: string;

    constructor(config: Configuration) {
        this.config = config;
        const {
            signalUrl,
            token,
        } = config;
        const [host, path] = splitUrl(signalUrl);
        this.emitter = new Emittery()
        this.ignoreOffer = false;
        this.pendingCandidates = [];
        this.onTrasksCallbacks = [];
        this.sdpMsgId = 1;
        this.negMux = new Mutex();
        this.peer = this.createPeer();
        this._id = uuidv4();
        this.socket = io(host, {
            auth: {
                token,
                id: this._id,
            },
            path,
            autoConnect: false,
            reconnection: true,
            reconnectionDelay: 500,
            rememberUpgrade: true,
        });
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
        })
        this.socket.on('error', (msg: ErrorMessage) => {
            this.emitter.emit('error', {
                name: 'error',
                data: msg,
            });
            this.logger().error(`Received${msg.fatal ? " fatal " : " "}error ${msg.msg} because of ${msg.cause}`);
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
        this.socket.on("sdp", (msg: SdpMessage, ark?: Ack) => {
            this.ack(ark);
            this.emitter.emit('sdp', {
                name: 'sdp',
                data: msg,
            });
            // const offerCollision = msg.type === "offer" 
            // && (this.makingOffer || peer.signalingState !== "stable");
            // const { polite = true } = this.config;
            // this.ignoreOffer = !polite && offerCollision;
            // if (this.ignoreOffer) {
            //     return;
            // }
            // await peer.setRemoteDescription({
            //     type: msg.type,
            //     sdp: msg.sdp,
            // });
            // console.log(`[${this.config.name}]:`)
            // console.log(peer.remoteDescription.sdp)
            // for (const pending of this.pendingCandidates) {
            //     await this.addCandidate(peer, pending);
            // }
            // if (msg.type === 'offer') {
            //     await peer.setLocalDescription();
            //     const desc = peer.localDescription
            //     const send_msg: SdpMessage = {
            //         type: desc.type,
            //         sdp: desc.sdp,
            //     };
            //     this.socket.emit("sdp", send_msg);
            // }
        });
        this.socket.on("candidate", async (msg: CandidateMessage, ark?: Ack) => {
            this.ack(ark);
            if (!this.peer.remoteDescription) {
                this.pendingCandidates.push(msg);
                return
            }
            await this.addCandidate(this.peer, msg);
        })
        // this.socket.on('stream', (msg: TrackMessage, ark?: Ark) => {
        //     if (msg.op == "add") {
        //         for (const track of msg.tracks) {
        //             console.log(`Add stream with id ${track.id} and stream id ${track.streamId}`);
        //         }
        //         this.tracks.push(...msg.tracks);
        //     } else {
        //         for (const track of msg.tracks) {
        //             console.log(`Remove stream with id ${track.id} and stream id ${track.streamId}`);
        //         }
        //         this.tracks = this.tracks.filter(tr => {
        //             for (const _tr of msg.tracks) {
        //                 if (tr.globalId == _tr.globalId && tr.id == _tr.id && tr.streamId == _tr.streamId) {
        //                     return false
        //                 }
        //             }
        //             return true
        //         });
        //     }
        //     for (const cb of this.onTrasksCallbacks) {
        //         cb({
        //             tracks: this.tracks,
        //             add: msg.op == 'add' ? msg.tracks : [],
        //             remove: msg.op == 'remove' ? msg.tracks : []
        //         });
        //     }
        //     if (ark) {
        //         ark()
        //     }
        // });
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
        // peer.onnegotiationneeded = async () => {
        //     try {
        //         this.makingOffer = true;
        //         await peer.setLocalDescription();
        //         const desc = peer.localDescription;
        //         const msg: SdpMessage = {
        //             msgId: this.nextSdpMsgId(),
        //             type: desc.type,
        //             sdp: desc.sdp,
        //         }
        //         this.socket.emit("sdp", msg)
        //     } catch (err) {
        //         console.error(err);
        //     } finally {
        //         this.makingOffer = false;
        //     }
        // };
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
        }
        peer.onconnectionstatechange = () => {
            this.logger().log(`connection state changed to ${peer.connectionState}`);
            this.emitter.emit('connectState', {
                name: 'connectState',
                data: this.peer.connectionState,
            });
        }
        peer.oniceconnectionstatechange = () => {
            this.logger().log(`ice connection state changed to ${peer.iceConnectionState}`);
        }
        peer.onsignalingstatechange = () => {
            this.logger().log(`signaling state changed to ${peer.signalingState}`);
        }
        peer.onicegatheringstatechange = () => {
            this.logger().log(`ice gathering state changed to ${peer.signalingState}`);
        }
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
        }
        return peer;
    }

    private makeSureSocket = async (stopEmitter?: Emittery<StopEmitEventMap>) => {
        if (this.socket.connected) {
            this.logger().debug('already connected.');
            return
        }
        this.logger().info("start connect socket...");
        this.socket.connect();
        const { data } = await Promise.race([
            this.emitter.once(['connect', 'disconnect']),
            stopEmitter ? stopEmitter.events('stop').next().then(() => {
                this.logger().debug('Received stop msg, it means timeout.');
                throw new TimeOutError();
            }) : new Promise<NamedEvent<'', SocketConnectState>>(() => {}),
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
            if (msg.op == "end") {
                this.logger().log(`Received candidate completed`)
                await peer.addIceCandidate();
            } else {
                this.logger().log(`Received candidate ${msg.candidate.candidate}`)
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
            if (pos == -1) {
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
                if (t.mid == bindId) {
                    return t;
                }
            }
            return null;
        }
    }

    join = async (...rooms: string[]) => {
        await this.makeSureSocket();
        const res = await this.socket.emitWithAck('join', {
            rooms,
        });
    }

    leave = async (...rooms: string[]) => {
        await this.makeSureSocket();
        await this.socket.emitWithAck('leave', {
            rooms,
        });
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
    }

    private waitForStableConnectionState = async () => {
        return await this.waitForEvt(
            'connectState',
            (evt) => evt.data === 'failed' || evt.data === 'closed' || evt.data === 'connected',
            () => this.peer.connectionState === 'failed' || this.peer.connectionState  === 'closed' || this.peer.connectionState === 'connected',
        );
    }

    private negotiate = async (sdpId: number, active: boolean, sdpEvts: AsyncIterableIterator<NamedEvent<'sdp', SdpMessage>> | null = null, stopEvts: AsyncIterableIterator<NamedEvent<'stop', undefined>> | null = null) => {
        const peer = this.peer;
        await this.waitForNotConnecting();
        if (peer.connectionState == 'closed') {
            throw ERR_PEER_CLOSED;
        }
        if (active) {
            if (sdpEvts !== null) {
                throw new Error(`sdpEvts should be null when active is true.`);
            }
            const offer = await peer.createOffer();
            await peer.setLocalDescription(offer);
            const desc = peer.localDescription;
            this.logger().debug(`create offer and set local desc`);
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
                        if (evt.data.mid == sdpId) {
                            if (evt.data.type == 'answer' || evt.data.type == 'pranswer') {
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
                if (sdpMsg.type == 'answer') {
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
                    if (evt.data.mid == sdpId) {
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

    publish = async (stream: LocalStream, timeout: number = 0) => {
        const cleaner = {
            stop: false,
            stopEmitter: new Emittery<StopEmitEventMap>(),
        };
        const timeoutHandler = {} as { handler: any };
        if (this.negMux.isLocked()) {
            this.logger().debug('There is another publish or subscribe task running, wait it finished.');
        }
        const task = this.negMux.runExclusive(async () => {
            await this.makeSureSocket(cleaner.stopEmitter);
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
            if (tracks.length == 0) {
                return "";
            }
            const cleanTracks = () => {
                if (timeoutHandler.handler) {
                    clearTimeout(timeoutHandler.handler);
                }
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
                const { id } = await this.socket.timeout(timeout).emitWithAck('publish', {
                    op: PUB_OP_ADD,
                    tracks,
                });
                pubId = id;
            } catch (e) {
                throw new TimeOutError();
            }
            const cleanAll = async () => {
                cleanTracks();
                await this._unpublish(pubId);
            }
            this.logger().debug(`accept publish id ${pubId}`);
            const sdpId = this.nextSdpMsgId();
            this.logger().debug(`gen sdp id ${sdpId}`);
            let stopEvt = cleaner.stopEmitter.events('stop');
            if (cleaner.stop) {
                await cleanAll();
                return "";  
            }
            const evts = combineAsyncIterable([this.emitter.events(['published', 'connectState', 'error']), stopEvt]);
            try {
                stopEvt = cleaner.stopEmitter.events('stop');
                if (cleaner.stop) {
                    await cleanAll();
                    return "";  
                }
                await this.negotiate(sdpId, true, null, stopEvt);
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
                } else if (evt.name === 'error') {
                    await evts.return();
                    await cleanAll();
                    throw new ServerError(evt.data);
                } else {
                    if (evt.data.track.pubId == pubId) {
                        const t = this.findTransceiverByBindId(evt.data.track.bindId);
                        if (t == null) {
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
                        if (pubNum == tracks.length) {
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
            if (timeoutHandler.handler) {
                clearTimeout(timeoutHandler.handler);
            }
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
                    new Promise<string>((_, reject) => {
                        timeoutHandler.handler = setTimeout(() => {
                            this.logger().warn('publish timeout.');
                            reject(new TimeOutError());
                        }, timeout);
                    }),
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
                        await this._unpublish(resHandler.pubId);
                    }
                }
                throw e;
            }
        } else {
            return task;
        }
    };

    private _unpublish = async (pubId: string) => {
        const res = await this.socket.emitWithAck('publish', {
            op: PUB_OP_REMOVE,
            id: pubId,
        });
        return res.id !== "";
    };

    unpublish = async (pubId: string) => {
        return this.negMux.runExclusive(() => {
            return this._unpublish(pubId);
        });
    }

    private checkTrack = (track: Track, transceiver: RTCRtpTransceiver) => {
        if (track.bindId.startsWith("pos:")) {
            const transceivers = this.peer.getTransceivers();
            const pos = transceivers.indexOf(transceiver);
            if (pos == -1) {
                return false;
            } else {
                return track.bindId === `pos:${pos}`;
            }
        } else {
            return track.bindId === transceiver.mid;
        }
    }

    subscribe = async (pattern: Pattern, reqTypes: string[] = [], timeout: number = 0) => {
        const cleaner = {
            stop: false,
            stopEmitter: new Emittery<StopEmitEventMap>(),
        };
        const timeoutHandler = {} as { handler: any };
        if (this.negMux.isLocked()) {
            this.logger().debug('There is another publish or subscribe task running, wait it finished.');
        }
        const task = this.negMux.runExclusive(async () => {
            await this.makeSureSocket(cleaner.stopEmitter);
            if (cleaner.stop) {
                return {
                    subId: "",
                    stream: undefined as MediaStream,
                };
            }
            this.logger().debug('start subscribe');
            const sdpEvts = this.emitter.events('sdp');
            let stopEvt = cleaner.stopEmitter.events('stop');
            const subEvts = combineAsyncIterable([this.emitter.events(['subscribed', 'error']), stopEvt]);
            stopEvt = cleaner.stopEmitter.events('stop');
            const trackOrStateEvts = combineAsyncIterable([this.emitter.events(['track', 'connectState', 'error']), stopEvt]);
            this.logger().debug('send sub msg');
            let subId: string = '';
            try {
                const { id } = await this.socket.timeout(timeout).emitWithAck('subscribe', {
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
                if (timeoutHandler.handler) {
                    clearTimeout(timeoutHandler.handler);
                }
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
                    if (subEvt.data.subId == subId) {
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
                stopEvt = cleaner.stopEmitter.events('stop');
                if (cleaner.stop) {
                    await clean();
                    return {
                        subId: "",
                        stream: undefined as MediaStream,
                    };
                }
                await this.negotiate(sdpId, false, sdpEvts, stopEvt);
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
                    trackOrStateEvts.return();
                    await clean();
                    return {
                        subId: "",
                        stream: undefined as MediaStream,
                    };
                } else {
                    if (evt.data.fatal) {
                        trackOrStateEvts.return();
                        throw new ServerError(evt.data);
                    }
                }
                if (resolved.length == tracks.length) {
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
            if (timeoutHandler.handler) {
                clearTimeout(timeoutHandler.handler);
            }
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
                    new Promise<{ subId: string, stream: MediaStream }>((_, reject) => {
                        timeoutHandler.handler = setTimeout(() => {
                            this.logger().warn('subscribe timeout.');
                            reject(new TimeOutError());
                        }, timeout);
                    })
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

    onTracks = async (callback: OnTrack) => {
        this.onTrasksCallbacks.push(callback);
    }

    offTracks = async (callback: OnTrack) => {
        const i = this.onTrasksCallbacks.indexOf(callback);
        if (i != -1) {
            this.onTrasksCallbacks.splice(i, 1);
        }
    }
}