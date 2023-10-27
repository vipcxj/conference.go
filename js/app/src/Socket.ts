import { io } from "socket.io-client";

async function getToken(uid: string, uname: string, role: string, room: string, nonce: number) {
    const token: string = await fetch(`http://localhost:3100/token?uid=${uid}&uname=${uname}&role=${role}&room=${room}&nonce=${nonce}`).then(r => r.text())
    return token
}

export async function testSocket() {
    const token = await getToken("user0", 'user0', 'student', 'user0', 12345)
    const socket = io("http://localhost:8080", {
        path: '/socket.io',
        auth: {
            token,
        },
    });
    socket.connect()
    socket.send('hello')
    socket.on('error', (...args) => {
        console.log(`Got error event with args ${args.join(', ')}`);
    });
    socket.onAny((evt, ...args) => {
        let ark: (...args: any[]) => void = () => undefined;
        if (args.length > 0 && typeof(args[args.length - 1]) =='function') {
            ark = args[args.length - 1];
            args = args.slice(0, args.length - 1);
        }
        console.log(`Got event ${evt} with args ${args.join(', ')}`);
        ark();
    })
}