import "webrtc-adapter";
interface Track {
    globalId: string;
    id: string;
    streamId: string;
}
type Ark = (...args: any[]) => any;
interface TrackEvent {
    tracks: Track[];
    add: Track[];
    remove: Track[];
}
type OnTrack = (tracks: TrackEvent) => any;
export declare class ConferenceClient {
    private socket;
    private polite;
    private makingOffer;
    private ignoreOffer;
    private pendingCandidates;
    private peer;
    private tracks;
    private onTrasksCallbacks;
    private emitter;
    name: string;
    constructor(signalUrl: string, token: string, polite?: boolean);
    ark: (func?: Ark) => void;
    makeSurePeer: () => RTCPeerConnection;
    private addCandidate;
    publish: (stream: MediaStream) => Promise<void>;
    private resolveSubscribed;
    subscribeOne: (track: Track) => Promise<MediaStreamTrack>;
    subscribeMany: (tracks: Track[]) => Promise<{
        [key: string]: MediaStreamTrack;
    }>;
    onTracks: (callback: OnTrack) => Promise<void>;
    offTracks: (callback: OnTrack) => Promise<void>;
    private wait;
}
export {};
