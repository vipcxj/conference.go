import { ErrorMessage } from './message';

export const ERR_PEER_CLOSED = {};
export const ERR_PEER_DISCONNECTED = {};
export const ERR_PEER_FAILED = {};
export const ERR_THIS_IS_IMPOSSIBLE = {};

export class TimeOutError extends Error {
    cleaned: boolean;

    constructor() {
        super();
        Object.setPrototypeOf(this, TimeOutError.prototype);
        this.cleaned = false;
    }
}

export class ServerError extends Error {

    data: ErrorMessage;

    constructor(msg: ErrorMessage) {
        super(msg.msg);
        Object.setPrototypeOf(this, ServerError.prototype);
        this.data = msg;
    }
}

export class SocketCloseError extends Error {

    reason: string;

    constructor(reason: string) {
        super(reason);
        Object.setPrototypeOf(this, SocketCloseError.prototype);
        this.reason = reason;
    }
}