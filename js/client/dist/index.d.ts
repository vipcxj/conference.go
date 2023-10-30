import "webrtc-adapter";
type Ark = (...args: any[]) => any;
export declare class ConferenceClient {
    private socket;
    private polite;
    private makingOffer;
    private ignoreOffer;
    private pendingCandidates;
    private peer;
    private streams;
    constructor(signalUrl: string, token: string, polite?: boolean);
    ark: (func?: Ark) => void;
    makeSurePeer: () => RTCPeerConnection;
    private addCandidate;
    publish: (stream: MediaStream) => Promise<void>;
    private wait;
}
export {};
