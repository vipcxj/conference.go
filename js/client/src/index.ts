import "webrtc-adapter";
import { io, Socket } from 'socket.io-client';
import { Mutex } from 'async-mutex'
import Emittery from 'emittery';
import { v4 as uuidv4 } from 'uuid';

import PATTERN, { Labels, Pattern } from './pattern';
import { ERR_PEER_CLOSED, ERR_PEER_FAILED, ERR_PEER_DISCONNECTED, ERR_THIS_IS_IMPOSSIBLE } from './errors';
import { getLogger } from './log';

export const PT = PATTERN;

class TimeOutError {
    cleaned: boolean

    constructor() {
        this.cleaned = false;
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

interface TrackMessage extends SignalMessage {
    op: "add" | "remove";
    tracks: Track[];
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

type Ark = (...args: any[]) => any;

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
    join: (msg: JoinMessage, ark: (res: any) => void) => void
    leave: (msg: LeaveMessage, ark: (res: any) => void) => void
    subscribe: (msg: SubscribeAddMessage, ark: (res: SubscribeResultMessage) => void) => void;
    publish: (msg: PublishAddMessage, ark: (res: PublishResultMessage) => void) => void;
    sdp: (msg: SdpMessage) => void;
    candidate: (msg: CandidateMessage) => void;
    want: (msg: any) => void;
    state: (msg: any) => void;
    select: (msg: any) => void;
}

interface EventData {
    connect: undefined;
    connectState: RTCPeerConnectionState;
    track: [MediaStreamTrack, readonly MediaStream[], RTCRtpTransceiver];
    subscribed: SubscribedMessage;
    published: PublishedMessage;
    sdp: SdpMessage;
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
            },
            path,
            reconnection: true,
            reconnectionDelay: 500,
            extraHeaders: {
                'Signal-Id': this._id,
            },
            rememberUpgrade: true,
        });
        this.socket.on('connect', () => {
            this.emitter.emit('connect');
        });
        this.socket.on('error', (msg: ErrorMessage) => {
            this.logger().error(`Received${msg.fatal ? " fatal " : " "}error ${msg.msg} because of ${msg.cause}`)
        });
        this.socket.on('subscribed', (msg: SubscribedMessage) => {
            this.emitter.emit('subscribed', msg);
        });
        this.socket.on('published', (msg: PublishedMessage) => {
            this.emitter.emit('published', msg);
        });
        this.socket.on('want', (msg: any) => {
            this.socket.emit('want', msg);
        });
        this.socket.on('state', (msg: any) => {
            this.socket.emit('state', msg);
        });
        this.socket.on('select', (msg: any) => {
            this.socket.emit('select', msg);
        });
        this.socket.on("sdp", async (msg: SdpMessage, ark?: Ark) => {
            this.emitter.emit('sdp', msg);
            this.ark(ark);
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
        this.socket.on("candidate", async (msg: CandidateMessage, ark?: Ark) => {
            this.ark(ark);
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

    private ark = (func?: Ark) => {
        if (func) {
            func();
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
            this.socket.emit("candidate", msg);
        }
        peer.onconnectionstatechange = () => {
            this.logger().log(`connection state changed to ${peer.connectionState}`);
            this.emitter.emit('connectState', this.peer.connectionState);
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
            this.emitter.emit("track", [evt.track, evt.streams, evt.transceiver]);
            this.logger().log(`Received track ${evt.track.id} with stream id ${evt.streams[0].id}`)
        }
        return peer;
    }

    private makeSureSocket = async () => {
        if (this.socket.connected) {
            return
        }
        this.socket.connect();
        await this.emitter.once('connect');
    }

    private addCandidate = async (peer: RTCPeerConnection, msg: CandidateMessage) => {
        try {
            if (msg.op == "end") {
                await peer.addIceCandidate();
            } else {
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
            }
        }
    }

    private waitForNotConnecting = async () => {
        return this.waitForEvt(
            'connectState',
            (evt) => evt !== 'connecting',
            () => this.peer.connectionState !== 'connecting',
        );
    }

    private waitForNeitherNewNorConnecting = async () => {
        return this.waitForEvt(
            'connectState',
            (evt) => evt !== 'new' && evt !== 'connecting',
            () => this.peer.connectionState !== 'new' && this.peer.connectionState !== 'connecting',
        );
    }

    private negotiate = async (sdpId: number, active: boolean, sdpEvts: AsyncIterableIterator<SdpMessage> | null = null) => {
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
            sdpEvts = this.emitter.events('sdp')
            await this.socket.emit('sdp', {
                type: desc.type,
                sdp: desc.sdp,
                mid: sdpId,
            });
            while (true) {
                let sdpMsg: SdpMessage;
                for await (const evt of sdpEvts) {
                    if (evt.mid == sdpId) {
                        if (evt.type == 'answer' || evt.type == 'pranswer') {
                            this.logger().debug(`receive remote ${evt.type} sdp`);
                            sdpMsg = evt;
                            break;
                        } else {
                            throw new Error(`Expect an answer or pranswer, but got ${evt.type}. The sdp:\n${evt.sdp}`);
                        }
                    }
                }
                this.logger().debug('set remote desc');
                await peer.setRemoteDescription({
                    type: sdpMsg.type,
                    sdp: sdpMsg.sdp,
                });
                if (sdpMsg.type == 'answer') {
                    await sdpEvts.return();
                    break;
                }
                for (const pending of this.pendingCandidates) {
                    await this.addCandidate(peer, pending);
                }
                this.pendingCandidates = [];
            }
        } else {
            if (sdpEvts === null) {
                throw new Error(`sdpEvts should not be null when active is false.`);
            }
            let sdpMsg: SdpMessage;
            for await (const evt of sdpEvts) {
                if (evt.mid == sdpId) {
                    if (evt.type === 'offer') {
                        this.logger().debug(`receive remote ${evt.type} sdp`);
                        sdpMsg = evt;
                        await sdpEvts.return()
                        break;
                    } else {
                        throw new Error(`Expect an offer, but got ${evt.type}. The sdp:\n${evt.sdp}`);
                    }
                }
            }
            this.logger().debug('set remote desc');
            await peer.setRemoteDescription({
                type: sdpMsg.type,
                sdp: sdpMsg.sdp,
            });
            for (const pending of this.pendingCandidates) {
                await this.addCandidate(peer, pending);
            }
            this.pendingCandidates = [];
            this.logger().debug('create answer');
            const answer = await peer.createAnswer();
            this.logger().debug('send answer');
            await this.socket.emit('sdp', {
                type: 'answer',
                sdp: answer.sdp,
                mid: sdpId,
            });
            this.logger().debug('set local desc');
            await peer.setLocalDescription(answer);
        }
        await this.waitForNeitherNewNorConnecting();
        // if (this.peer.connectionState == 'closed') {
        //     throw ERR_PEER_CLOSED;
        // } else if (this.peer.connectionState == 'disconnected') {
        //     throw ERR_PEER_DISCONNECTED;
        // } else if (this.peer.connectionState == 'failed') {
        //     throw ERR_PEER_FAILED;
        // }
        // if (this.peer.connectionState != 'connected') {
        //     throw ERR_THIS_IS_IMPOSSIBLE;
        // }
    }

    publish = async (stream: LocalStream) => {
        return this.negMux.runExclusive(async () => {
            await this.makeSureSocket();
            const peer = this.peer;
            this.logger().debug('start publish');
            const tracks: TrackToPublish[] = [];
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
                this.logger().debug(`add track ${JSON.stringify(t)}`);
            }
            if (tracks.length == 0) {
                return
            }
            const sdpId = this.nextSdpMsgId();
            this.logger().debug(`gen sdp id ${sdpId}`);
            await this.negotiate(sdpId, true, null);
            const evts = this.emitter.events('published')
            const { id: pubId } = await this.socket.emitWithAck('publish', {
                op: PUB_OP_ADD,
                tracks,
            });
            this.logger().debug(`accept publish id ${pubId}`);
            let pubNum = 0;
            for await (const evt of evts) {
                if (evt.track.pubId == pubId) {
                    const t = this.findTransceiverByBindId(evt.track.bindId);
                    if (t == null) {
                        throw Error('receive a unknown published track');
                    }
                    if (!t.sender.track) {
                        throw Error('receive a invalid published track');
                    }
                    this.logger().debug(`track ${t.sender.track.id} is published`);
                    pubNum++;
                    if (pubNum == tracks.length) {
                        await evts.return();
                        break;
                    }
                }
            }
            this.logger().debug(`publish ${pubId} completed`);
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

    subscribe = async (pattern: Pattern, reqTypes: string[] = []) => {
        return this.negMux.runExclusive(async () => {
            await this.makeSureSocket();
            const sdpEvts = this.emitter.events('sdp');
            const subEvts = this.emitter.events('subscribed');
            const trackEvts = this.emitter.events('track');
            const { id: subId } = await this.socket.emitWithAck('subscribe', {
                op: SUB_OP_ADD,
                reqTypes,
                pattern,
            });
            let subedMsg: SubscribedMessage;
            for await (const subEvt of subEvts) {
                if (subEvt.subId == subId) {
                    subedMsg = subEvt;
                    await subEvts.return();
                    break;
                }
            }
            const { tracks, sdpId } = subedMsg;
            await this.negotiate(sdpId, false, sdpEvts);
            const resolved: Track[] = [];
            let stream: MediaStream;
            for await (const [_, streams, transceiver] of trackEvts) {
                for (const t of tracks) {
                    if (this.checkTrack(t, transceiver)) {
                        stream = streams[0]
                        resolved.push(t)
                        break
                    }
                }
                if (resolved.length == tracks.length) {
                    break
                }
            }
            return stream;
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