import React from "react"
import { ConferenceClient } from "conference.go/dist"

export function useDelay(f: (...args: any[]) => any) {
    React.useEffect(() => {
        const h = setTimeout(() => {
            f();
        }, 500);
        return () => {
            clearTimeout(h);
        }
    }, [f]);
}

async function getToken(uid: string, uname: string, role: string, room: string, nonce: number) {
    const token: string = await fetch(`http://localhost:3100/token?uid=${uid}&uname=${uname}&role=${role}&room=${room}&nonce=${nonce}`).then(r => r.text())
    return token
}

export async function testSocket() {
    const token0 = await getToken("user0", 'user0', 'student', 'room0', 12345)
    const client0 = new ConferenceClient("http://localhost:8080/socket.io", token0);
    client0.name = "client0"
    // const token1 = await getToken("user1", 'user1', 'student', 'room0', 12345)
    // const client1 = new ConferenceClient("http://localhost:8080/socket.io", token1);
    // client1.name = "client1"
    const stream = await navigator.mediaDevices.getUserMedia({
        video: true,
    });
    await client0.publish(stream);
    client0.onTracks(async ({ tracks }) => {
        const results = await client0.subscribeMany(tracks);
        for (const gid in results) {
            console.log(`subscribed ${gid} by client 0`);
        }
    });
    // client1.onTracks(async ({ tracks }) => {
    //     const results = await client1.subscribeMany(tracks);
    //     for (const gid in results) {
    //         console.log(`subscribed ${gid} by client 1`);
    //     }
    // });
}