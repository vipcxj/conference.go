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

interface SdpMessage {
    sdp: string;
}

interface CandidateMessage {
    op: "add" | "end";
    candidate?: RTCIceCandidateInit
}

interface StreamMessage {
    op: "add" | "remove"
}

export class ConferenceClient {
    private socket: Socket;
    private peer: RTCPeerConnection;

    constructor(signalUrl: string, token: string) {
        const [host, path] = splitUrl(signalUrl);
        this.socket = io(host, {
            auth: {
                token,
            },
            path,
            autoConnect: true,
        });
    }

    makeSurePeer = () => {
        if (!this.peer) {
            const peer = new RTCPeerConnection();
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
            const self = this;
            const candidateListener = function (candidate: CandidateMessage, ark?: (response: any) => void) {
                if (candidate.op == "end") {
                    peer.addIceCandidate(undefined);
                    self.socket.off(this);
                } else {
                    peer.addIceCandidate(candidate.candidate);
                }
            };
            this.socket.on("candidate", candidateListener);
            this.peer = peer;
        }
        return this.peer
    }

    publish = async (stream: MediaStream) => {
        this.socket.connect()
        const peer = this.makeSurePeer()
        stream.getTracks().forEach(track => {
            peer.addTrack(track, stream);
        });
        const offer = await peer.createOffer();
        await peer.setLocalDescription(offer);
        const waitForAnswer = this.wait<SdpMessage>("answer");
        this.socket.emit("offer", {
            sdp: peer.localDescription.sdp
        });
        const { sdp } = await waitForAnswer
        await peer.setRemoteDescription({ type: "answer", sdp });
        console.log("published.");
    }

    private wait = <R>(evt: string, { arkData, timeout }: { arkData?: any, timeout?: number } = {}): Promise<R> => {
        return new Promise((resolve, reject) => {
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