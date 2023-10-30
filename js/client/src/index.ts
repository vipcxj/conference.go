import "webrtc-adapter";
import { io, Socket } from 'socket.io-client';

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

interface Stream {
    id: string;
    streamId: string;
}

interface StreamMessage extends SignalMessage {
    op: "add" | "remove";
    stream: Stream;
}

type Ark = (...args: any[]) => any;

export class ConferenceClient {
    private socket: Socket;
    private polite: boolean;
    private makingOffer: boolean;
    private ignoreOffer: boolean;
    private pendingCandidates: CandidateMessage[];
    private peer: RTCPeerConnection;
    private streams: Stream[];

    constructor(signalUrl: string, token: string, polite: boolean = true) {
        const [host, path] = splitUrl(signalUrl);
        this.makingOffer = false;
        this.ignoreOffer = false;
        this.polite = polite;
        this.pendingCandidates = [];
        this.streams = [];
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
        this.socket.on('stream', (msg: StreamMessage, ark?: Ark) => {
            if (msg.op == "add") {
                console.log(`Add stream with id ${msg.stream.id} and stream id ${msg.stream.streamId}`);
                this.streams.push(msg.stream);
            } else {
                console.log(`Remove stream with id ${msg.stream.id} and stream id ${msg.stream.streamId}`);
                this.streams = this.streams.filter(st => st.id == msg.stream.id && st.streamId == msg.stream.streamId);
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