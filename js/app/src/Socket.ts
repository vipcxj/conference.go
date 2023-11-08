import React from "react"
import { ConferenceClient, PT } from "conference.go/dist"

export function useDelay(f: (...args: any[]) => any, ...args: any[]) {
    React.useEffect(() => {
        const h = setTimeout(() => {
            f(...args);
        }, 500);
        return () => {
            clearTimeout(h);
        }
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [f, ...args]);
}

async function getToken(uid: string, uname: string, role: string, room: string, nonce: number) {
    const token: string = await fetch(`http://localhost:3100/token?uid=${uid}&uname=${uname}&role=${role}&room=${room}&nonce=${nonce}`).then(r => r.text())
    return token
}

export async function testSocket(inputVideoRef: React.RefObject<HTMLVideoElement>, outputVideoRefs: Array<React.RefObject<HTMLVideoElement>>) {
    const token0 = await getToken("user0", 'user0', 'student', 'room0', 12345)
    const client0 = new ConferenceClient("http://localhost:8080/socket.io", token0);
    client0.name = "client0"
    const token1 = await getToken("user1", 'user1', 'student', 'room0', 12345)
    const client1 = new ConferenceClient("http://localhost:8080/socket.io", token1);
    client1.name = "client1"
    const stream = await navigator.mediaDevices.getUserMedia({
        video: true,
    });
    if (inputVideoRef.current) {
        inputVideoRef.current.srcObject = stream;
    }
    await client0.publish({
        stream,
        labels: {
            user: "user0",
            role: "student",
        },
    });
    const ss0 = await client0.subscribe(PT.All(PT.TrackTypeIn("video"), PT.LabelsAllMatch({
        user: "user0",
        role: "student",
    })));
    if (outputVideoRefs[0].current) {
        outputVideoRefs[0].current.srcObject = ss0;
    }
    const ss1 = await client1.subscribe(PT.All(PT.TrackTypeIn("video"), PT.LabelsAllMatch({
        user: "user0",
        role: "student",
    })));
    if (outputVideoRefs[1].current) {
        outputVideoRefs[1].current.srcObject = ss1;
    }

    // client1.onTracks(async ({ tracks }) => {
    //     const results = await client1.subscribeMany(tracks);
    //     for (const gid in results) {
    //         console.log(`subscribed ${gid} by client 1`);
    //     }
    // });
}