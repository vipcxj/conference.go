import "webrtc-adapter";
import { io, Socket } from 'socket.io-client';
import Emittery from 'emittery';

import PATTERN, { Labels, Pattern } from './pattern';

export const PT = PATTERN;

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
    msgId?: number
}

interface SdpMessage extends SignalMessage {
    type: RTCSdpType;
    sdp: string;
    
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
    subscribe: (msg: SubscribeAddMessage, ark: (res: SubscribeResultMessage) => void) => void;
    publish: (msg: PublishAddMessage, ark: (res: PublishResultMessage) => void) => void;
    sdp: (msg: SdpMessage) => void;
    candidate: (msg: CandidateMessage) => void;
    want: (msg: any) => void;
    state: (msg: any) => void;
    select: (msg: any) => void;
}

interface EventData {
    track: [MediaStreamTrack, readonly MediaStream[], RTCRtpTransceiver];
    subscribed: SubscribedMessage;
    published: PublishedMessage;
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
    private makingOffer: boolean;
    private ignoreOffer: boolean;
    private pendingCandidates: CandidateMessage[];
    private peer: RTCPeerConnection;
    private onTrasksCallbacks: OnTrack[];
    private emitter: Emittery<EventData>;
    private sdpMsgId: number;

    constructor(config: Configuration) {
        this.config = config;
        const {
            signalUrl,
            token,
        } = config;
        const [host, path] = splitUrl(signalUrl);
        this.emitter = new Emittery()
        this.makingOffer = false;
        this.ignoreOffer = false;
        this.pendingCandidates = [];
        this.onTrasksCallbacks = [];
        this.sdpMsgId = 0;
        this.socket = io(host, {
            auth: {
                token,
            },
            path,
            autoConnect: true,
        });
        this.socket.on('error', (msg: ErrorMessage) => {
            console.error(`Received${msg.fatal ? " fatal " : " "}error ${msg.msg} because of ${msg.cause}`)
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

    nextSdpMsgId = () => {
        return ++this.sdpMsgId;
    }

    ark = (func?: Ark) => {
        if (func) {
            func();
        }
    }

    makeSurePeer = () => {
        if (!this.peer) {
            const peer = new RTCPeerConnection(this.config.rtcConfig);
            peer.onnegotiationneeded = async () => {
                try {
                    this.makingOffer = true;
                    await peer.setLocalDescription();
                    const desc = peer.localDescription;
                    const msg: SdpMessage = {
                        msgId: this.nextSdpMsgId(),
                        type: desc.type,
                        sdp: desc.sdp,
                    }
                    this.socket.emit("sdp", msg)
                } catch (err) {
                    console.error(err);
                } finally {
                    this.makingOffer = false;
                }
            };
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
                console.log(`[${this.config.name}] connection state changed to ${peer.connectionState}`);
            }
            peer.oniceconnectionstatechange = () => {
                console.log(`[${this.config.name}] ice connection state changed to ${peer.iceConnectionState}`);
            }
            peer.onsignalingstatechange = () => {
                console.log(`[${this.config.name}] signaling state changed to ${peer.signalingState}`);
            }
            peer.onicegatheringstatechange = () => {
                console.log(`[${this.config.name}] ice gathering state changed to ${peer.signalingState}`);
            }
            peer.ontrack = async (evt) => {
                evt.track.onmute = (evt0) => {
                    console.log(`The remote track ${evt.track.id}/${evt.transceiver.mid} is muted`);
                }
                evt.track.onunmute = (evt0) => {
                    console.log(`The remote track ${evt.track.id}/${evt.transceiver.mid} is unmuted`);
                }
                evt.track.onended = (evt0) => {
                    console.log(`The remote track ${evt.track.id}/${evt.transceiver.mid} is ended`);
                }
                this.emitter.emit("track", [evt.track, evt.streams, evt.transceiver]);
                console.log(`Received track ${evt.track.id} with stream id ${evt.streams[0].id}`)
            }
            this.socket.on("sdp", async (msg: SdpMessage, ark?: Ark) => {
                this.ark(ark);
                const offerCollision = msg.type === "offer" 
                && (this.makingOffer || peer.signalingState !== "stable");
                const { polite = true } = this.config;
                this.ignoreOffer = !polite && offerCollision;
                if (this.ignoreOffer) {
                    return;
                }
                await peer.setRemoteDescription({
                    type: msg.type,
                    sdp: msg.sdp,
                });
                console.log(`[${this.config.name}]:`)
                console.log(peer.remoteDescription.sdp)
                for (const pending of this.pendingCandidates) {
                    await this.addCandidate(peer, pending);
                }
                if (msg.type === 'offer') {
                    await peer.setLocalDescription();
                    const desc = peer.localDescription
                    const send_msg: SdpMessage = {
                        type: desc.type,
                        sdp: desc.sdp,
                    };
                    this.socket.emit("sdp", send_msg);
                }
            });
            this.socket.on("candidate", async (msg: CandidateMessage, ark?: Ark) => {
                this.ark(ark);
                if (!peer.remoteDescription) {
                    this.pendingCandidates.push(msg);
                    return
                }
                await this.addCandidate(peer, msg);
            })
            this.peer = peer;
        }
        return this.peer
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

    getMid = (peer: RTCPeerConnection, transceiver: RTCRtpTransceiver): string => {
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

    publish = async (stream: LocalStream) => {
        this.socket.connect()
        const peer = this.makeSurePeer()
        const tracks: TrackToPublish[] = [];
        for (const track of stream.stream.getTracks()) {
            const transceiver = peer.addTransceiver(track, {
                direction: 'sendrecv',
                streams: [stream.stream],
            });
            tracks.push({
                type: track.kind,
                bindId: this.getMid(peer, transceiver),
                sid: stream.stream.id,
                labels: stream.labels,
            });
        }
        if (tracks.length == 0) {
            return
        }
        const { id: pubId } = await this.socket.emitWithAck('publish', {
            op: PUB_OP_ADD,
            tracks,
        });
        let pubNum = 0;
        const evts = this.emitter.events('published')
        for await (const evt of evts) {
            if (evt.track.pubId == pubId) {
                pubNum++;
                if (pubNum == tracks.length) {
                    evts.return();
                    break;
                }
            }
        }
    }

    checkTrack = (track: Track, transceiver: RTCRtpTransceiver) => {
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
        this.socket.connect()
        const peer = this.makeSurePeer()
        const { id: subId } = await this.socket.emitWithAck('subscribe', {
            op: SUB_OP_ADD,
            reqTypes,
            pattern,
        });
        const subEvts = this.emitter.events('subscribed');
        let tracks: Track[]
        let sdpId: number = 0
        const trackEvts = this.emitter.events('track');
        for await (const subEvt of subEvts) {
            if (subEvt.subId == subId) {
                tracks = subEvt.tracks;
                sdpId = subEvt.sdpId;
                subEvts.return();
                break;
            }
        }
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