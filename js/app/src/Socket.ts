import { io } from "socket.io-client";

async function getToken(uid: string, uname: string, role: string, room: string, nonce: number) {
    const token: string = await fetch(`http://localhost:3100/token?uid=${uid}&uname=${uname}&role=${role}&room=${room}&nonce=${nonce}`).then(r => r.text())
    return token
}

export async function testSocket() {
    const token = await getToken("user0", 'user0', 'student', 'user0', 12345)
    const socket = io("http://localhost:8080/socket.io", {
        auth: {
            token,
        },
    });
    socket.connect()
    socket.onAny((evt, ...args) => {
        console.log(`Got event ${evt} with args ${args.join(', ')}`)
    })
}