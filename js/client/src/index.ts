import "webrtc-adapter";
import { io, Socket } from 'socket.io-client';
import Emittery from 'emittery';

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

interface SignalMessage {
    to?: string
}

interface SdpMessage extends SignalMessage {
    type: RTCSdpType;
    sdp: string;
}

interface CandidateMessage extends SignalMessage {
    op: "add" | "end";
    candidate?: RTCIceCandidateInit;
}

interface ErrorMessage extends SignalMessage {
    msg: string;
    cause: string;
    fatal: boolean;
}

interface Track {
    globalId: string
    id: string;
    streamId: string;
}

interface TrackMessage extends SignalMessage {
    op: "add" | "remove";
    tracks: Track[];
}

interface SubscribeMessage extends SignalMessage {
    tracks: Track[];
}

interface SubscribedMessage extends SignalMessage {
    tracks: Track[];
}

type Ark = (...args: any[]) => any;

interface TrackEvent {
    tracks: Track[];
    add: Track[];
    remove: Track[];
}

type OnTrack = (tracks: TrackEvent) => any;

interface EventData {
    "subscribed": [MediaStreamTrack, readonly MediaStream[]];
}

export class ConferenceClient {
    private socket: Socket;
    private polite: boolean;
    private makingOffer: boolean;
    private ignoreOffer: boolean;
    private pendingCandidates: CandidateMessage[];
    private peer: RTCPeerConnection;
    private tracks: Track[];
    private onTrasksCallbacks: OnTrack[];
    private emitter: Emittery<EventData>;
    name: string;

    constructor(signalUrl: string, token: string, polite: boolean = true) {
        const [host, path] = splitUrl(signalUrl);
        this.name = '';
        this.emitter = new Emittery()
        this.makingOffer = false;
        this.ignoreOffer = false;
        this.polite = polite;
        this.pendingCandidates = [];
        this.tracks = [];
        this.onTrasksCallbacks = [];
        this.socket = io(host, {
            auth: {
                token,
            },
            path,
            autoConnect: true,
        });
        this.socket.on('error', (msg: ErrorMessage, ark?: Ark) => {
            console.error(`Received${msg.fatal ? " fatal " : " "}error ${msg.msg} because of ${msg.cause}`)
            if (ark) {
                ark()
            }
        });
        this.socket.on('stream', (msg: TrackMessage, ark?: Ark) => {
            if (msg.op == "add") {
                for (const track of msg.tracks) {
                    console.log(`Add stream with id ${track.id} and stream id ${track.streamId}`);
                }
                this.tracks.push(...msg.tracks);
            } else {
                for (const track of msg.tracks) {
                    console.log(`Remove stream with id ${track.id} and stream id ${track.streamId}`);
                }
                this.tracks = this.tracks.filter(tr => {
                    for (const _tr of msg.tracks) {
                        if (tr.globalId == _tr.globalId && tr.id == _tr.id && tr.streamId == _tr.streamId) {
                            return false
                        }
                    }
                    return true
                });
            }
            for (const cb of this.onTrasksCallbacks) {
                cb({
                    tracks: this.tracks,
                    add: msg.op == 'add' ? msg.tracks : [],
                    remove: msg.op == 'remove' ? msg.tracks : []
                });
            }
            if (ark) {
                ark()
            }
        });
        this.socket.onAny((evt: string, ...args) => {
            if (['error', 'stream', 'sdp', 'candidate'].indexOf(evt) !== -1) {
                return;
            }
            let ark = args[args.length - 1];
            const hasArk = typeof ark == 'function';
            if (hasArk) {
                args = args.slice(0, args.length - 1);
            } else {
                ark = () => undefined
            }
            if (evt == "want" || evt == "state") {
                this.socket.emit(evt, ...args);
                return;
            }
            console.log(`Received event ${evt} with args ${args.join(",")}`);
            ark();
        })
    }

    ark = (func?: Ark) => {
        if (func) {
            func();
        }
    }

    makeSurePeer = () => {
        if (!this.peer) {
            const peer = new RTCPeerConnection();
            peer.onnegotiationneeded = async () => {
                try {
                    this.makingOffer = true;
                    await peer.setLocalDescription();
                    const desc = peer.localDescription;
                    const msg: SdpMessage = {
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
            peer.ontrack = async (evt) => {
                this.emitter.emit("subscribed", [evt.track, evt.streams]);
                console.log(`Received track ${evt.track.id} with stream id ${evt.streams[0].id}`)
            }
            this.socket.on("sdp", async (msg: SdpMessage, ark?: Ark) => {
                this.ark(ark);
                const offerCollision = msg.type === "offer" 
                && (this.makingOffer || peer.signalingState !== "stable");
                this.ignoreOffer = !this.polite && offerCollision;
                if (this.ignoreOffer) {
                    return;
                }
                await peer.setRemoteDescription({
                    type: msg.type,
                    sdp: msg.sdp,
                });
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

    publish = async (stream: MediaStream) => {
        this.socket.connect()
        const peer = this.makeSurePeer()
        stream.getTracks().forEach(track => {
            peer.addTrack(track, stream);
        });
    }

    private resolveSubscribed = (track: Track): MediaStreamTrack | null => {
        if (!this.peer) {
            return null;
        }
        for (const receiver of this.peer.getReceivers()) {
            if (receiver.track.id == track.id) {
                return receiver.track;
            }
        }
        return null;
    }

    subscribeOne =async (track: Track): Promise<MediaStreamTrack> => {
        const result = await this.subscribeMany([track]);
        return result[track.globalId];
    }

    subscribeMany = async (tracks: Track[]): Promise<{ [key:string]: MediaStreamTrack }> => {
        this.socket.connect()
        this.makeSurePeer()
        const msg: SubscribeMessage = {
            tracks,
        };
        this.socket.emit('subscribe', msg)
        const subscribed: SubscribedMessage = await this.wait('subscribed')
        const result: { [key:string]: MediaStreamTrack } = {}
        const pendingResult: Track[] = []
        let resolved = 0;
        for (const track of subscribed.tracks) {
            const st = this.resolveSubscribed(track);
            if (st) {
                result[track.globalId] = st;
                ++resolved;
            } else {
                pendingResult.push(track);
            }
        }
        if (resolved == subscribed.tracks.length) {
            return result;
        }
        for await (const msg of this.emitter.events('subscribed')) {
            const [st, _] = msg;
            for (const track of pendingResult) {
                if (st.id == track.id) {
                    result[track.globalId] = st;
                    ++resolved;
                }
            }
            if (resolved == subscribed.tracks.length) {
                return result;
            }
        }
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

    private wait = <R>(evt: string, { arkData, timeout }: { arkData?: any, timeout?: number } = {}): Promise<R> => {
        return new Promise((resolve) => {
            this.socket.once(evt, (...args) => {
                let ark: (...args: any[]) => void = () => undefined;
                if (args.length > 0 && typeof(args[args.length - 1]) =='function') {
                    ark = args[args.length - 1];
                    args = args.slice(0, args.length - 1);
                }
                if (arkData) {
                    ark(arkData);
                } else {
                    ark()
                }
                if (args.length > 1) {
                    console.warn(`Too many response data: ${args.join(", ")}`);
                }
                if (args.length == 0) {
                    resolve(undefined);
                } else {
                    let res: R;
                    try {
                        res = JSON.parse(args[0]);
                    } catch(e) {
                        res = args[0];
                    }
                    resolve(res);
                }
            });
        })
    }
}