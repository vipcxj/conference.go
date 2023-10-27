import { io, Socket } from 'socket.io-client';

function splitUrl(url: string) {
    const pos = url.indexOf('/');
    if (pos != -1) {
        return [url.substring(0, pos), url.substring(pos + 1)]
    } else {
        return [url, '']
    }
}

export class ConferenceClient {
    private socket: Socket;

    constructor(signalUrl: string, token: string) {
        const [host, path] = splitUrl(signalUrl);
        this.socket = io(host, {
            auth: {
                token,
            },
            path,
        });
    }

    connect = () => {
        this.socket.connect()
    }

    
}