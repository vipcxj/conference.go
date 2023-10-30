import { ConferenceClient } from "conference.go/dist"

async function getToken(uid: string, uname: string, role: string, room: string, nonce: number) {
    const token: string = await fetch(`http://localhost:3100/token?uid=${uid}&uname=${uname}&role=${role}&room=${room}&nonce=${nonce}`).then(r => r.text())
    return token
}

export async function testSocket() {
    const token = await getToken("user0", 'user0', 'student', 'user0', 12345)
    const client = new ConferenceClient("http://localhost:8080/socket.io", token);
    const stream = await navigator.mediaDevices.getUserMedia({
        video: true,
    });
    await client.publish(stream);
}