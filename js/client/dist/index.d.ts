export declare class ConferenceClient {
    private socket;
    private peer;
    constructor(signalUrl: string, token: string);
    connect: () => Promise<void>;
    private wait;
}
