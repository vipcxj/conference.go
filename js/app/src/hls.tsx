
import React from 'react'
import Hls from 'hls.js'
import { TIME_OUT } from './Socket'

export interface HlsProps {
    indexUrl: string;
    delay: number;
}

class SleepHandle {
    delay: number;
    th?: any;
    reject?: (reason: any) => void;

    constructor(delay: number) {
        this.delay = delay;
    }

    cancel = () => {
        if (this.th) {
            clearTimeout(this.th);
            this.th = undefined;
        }
        if (this.reject) {
            this.reject(TIME_OUT);
            this.reject = undefined;
        }
    }
}

const sleep = (handle: SleepHandle): Promise<undefined> => {
    return new Promise((resolve, reject) => {
        handle.th = setTimeout(() => {
            resolve(undefined);
        }, handle.delay);
        handle.reject = reject;
    })
}

async function bindHls(videoRef: React.RefObject<HTMLVideoElement>, indexUrl: string, sleepHandle?: SleepHandle) {
    if (sleepHandle) {
        await sleep(sleepHandle);
    }

    const video = videoRef.current;
    if (!video) {
        return null
    }
    const hls = new Hls({
        autoStartLoad: true,
    });
    hls.on(Hls.Events.MEDIA_ATTACHED, () => {
        console.log('video and hls.js are now bound together !');
    });
    hls.on(Hls.Events.MANIFEST_PARSED, (event, data) => {
        console.log(
          'manifest loaded, found ' + data.levels.length + ' quality level',
        );
    });
    hls.loadSource(indexUrl);
    hls.attachMedia(video);
    return hls;
}

export function HlsPlayer(props: HlsProps) {
    const { indexUrl, delay } = props;
    const videoRef = React.useRef<HTMLVideoElement>(null)
    React.useEffect(() => {
        const sleepHandle = new SleepHandle(delay);
        const hlsPromise = bindHls(videoRef, indexUrl, sleepHandle);
        return () => {
            sleepHandle.cancel();
            hlsPromise.then((hls) => {
                if (hls) {
                    hls.destroy();
                }
            }).catch(() => {});
        };
    }, [indexUrl, delay]);

    if (!Hls.isSupported()) {
        return <div>HLS is not supported at this browser.</div>
    }
    return (
        <video ref={videoRef} controls autoPlay></video>
    )
}