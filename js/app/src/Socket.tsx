import { useRef, useEffect, useState } from "react"
import { ConferenceClient, PT, TimeOutError } from "conference.go/lib"
import type { Labels } from 'conference.go/lib/pattern'

export function useOnce(effect: () => void | Promise<void>, readyFunc: () => boolean = () => true) {
    const initialized = useRef(false)
    const ready = readyFunc();
    useEffect(() => {
      if (!initialized.current && ready) {
        initialized.current = true
        effect()
      }
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [ready]);
}

export function useCreateOnce<T>(factory: () => Promise<T>): T | undefined {
    const [value, setValue] = useState<T>()
    useOnce(async () => {
        const obj = await factory();
        setValue(obj);
    });
    return value;
}

export interface VideoProps {
    stream?: MediaStream;
    rtcConfig?: RTCConfiguration;
    auth: {
        uid: string;
        uname: string;
        role: string;
        room: string;
    };
    publish: {
        labels: Labels;
    };
    subscribe: {
        labels: Labels;
    };
    name?: string;
    signalHost: string
    authHost: string
}

export const TIME_OUT = {};

async function withTimeout<T>(promise: Promise<T>, ms: number): Promise<T> {
    return Promise.race([promise, new Promise<T>((resolve, reject) => {
        setTimeout(() => {
            reject(TIME_OUT);
        }, ms);
    })]);
}

export const Video = (pros: VideoProps) => {
    const {
        name,
        auth,
        publish,
        subscribe,
        signalHost = "http://localhost:8080",
        authHost = "http://localhost:3100",
        stream,
        rtcConfig,
    } = pros;
    const { uid, uname, role, room } = auth;
    const videoRef = useRef<HTMLVideoElement>(null);
    const [client, setClient] = useState<ConferenceClient>();
    useOnce(async () => {
        const nonce = Math.floor(Math.random() * 100000);
        const token = await fetch(`${authHost}/token?key=${uid}&uid=${uid}&uname=${uname}&role=${role}&room=${room}&nonce=${nonce}&autojoin=true`).then(r => r.text());
        const client = new ConferenceClient({
            name,
            signalUrl: `${signalHost}/socket.io`,
            token,
            rtcConfig,
        });
        setClient(client);
        while (true) {
            try {
                await client.publish({
                    stream: stream!,
                    labels: publish.labels,
                    constraints: [{
                        type: 'video',
                        codec: {
                            profileLevelId: '42e01f',
                        },
                    }],
                }, 30000);
                break;
            } catch (e) {
                if (!(e instanceof TimeOutError)) {
                    throw e;
                } else {
                    client.logger().warn("publish timeout, republish.");
                }
            }
        }
        while (true) {
            try {
                const { stream: ss } = await client.subscribe(PT.All(
                    PT.TrackTypeIn('video'),
                    PT.LabelsAllMatch(subscribe.labels),
                ), undefined, 30000);
                if (videoRef.current) {
                    videoRef.current.srcObject = ss;
                }
                break;
            } catch (e) {
                if (!(e instanceof TimeOutError)) {
                    throw e;
                } else {
                    client.logger().warn("subscribe timeout, resubscribe.");
                }
            }
        }
    }, () => !!stream);
    return (
        <div className="Video-Container">
            <video className="Video" ref={videoRef} autoPlay controls/>
            <div className="Video-Title">
                {`${auth.room}, pub: ${publish.labels['key']}, sub: ${subscribe.labels['key']}`} <br />
                {`${client?.id()}`} <br/>
            </div>
        </div>
    );
}
