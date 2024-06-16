import { TimeOutError } from './errors';
import { NamedEvent } from './types';
import Emittery from 'emittery';

export interface StopEmitEventMap {
    stop: NamedEvent<'stop', undefined>;
}

export interface TimeoutEmitEventMap {
    timeout: NamedEvent<'timeout', undefined>;
}

export interface TimeoutHandler {
    handler?: any;
};

export class Stopper {
    private emitter: Emittery<StopEmitEventMap>;

    constructor() {
        this.emitter = new Emittery<StopEmitEventMap>();
    }

    getEmitter = () => {
        return this.emitter;
    }

    stop = () => {
        this.emitter.emit('stop', {
            name: 'stop',
            data: undefined,
        });
    }
}

export class Timeouter {
    private timeoutInMs: number;
    private start: number;
    private stoped: boolean;
    private timeoutHandlers: TimeoutHandler[];
    private emitters: Array<Emittery<StopEmitEventMap>>;

    constructor(timeoutInMs: number) {
        this.timeoutInMs = timeoutInMs;
        this.timeoutHandlers = [];
        this.start = Date.now();
        this.stoped = false;
    }

    elapsed = () => {
        return Date.now() - this.start;
    }

    left = () => {
        if (this.timeoutInMs < 0) {
            return this.timeoutInMs;
        }
        const l = this.timeoutInMs - this.elapsed();
        if (l < 0) {
            return 0;
        } else {
            return l;
        }
    };

    stop = () => {
        if (this.stoped || this.left() === 0) {
            return;
        }
        this.stoped = true;
        for(const handler of this.timeoutHandlers) {
            stopTimeoutHandler(handler);
        }
        this.timeoutHandlers = [];
        for(const emitter of this.emitters) {
            emitter.emit('stop', {
                name: 'stop',
                data: undefined,
            });
        }
        this.emitters = [];
    };

    stopEvt = () => {
        const emitter = new Emittery<StopEmitEventMap>();
        const left = this.left();
        if (this.stoped || left === 0) {
            emitter.emit('stop', {
                name: 'stop',
                data: undefined,
            });
        } else {
            const handler: TimeoutHandler = {};
            this.timeoutHandlers.push(handler);
            this.emitters.push(emitter);
            handler.handler = setTimeout(() => {
                let pos = this.emitters.indexOf(emitter);
                if (pos !== -1) {
                    this.emitters.splice(pos, 1);
                }
                pos = this.timeoutHandlers.indexOf(handler);
                if (pos !== -1) {
                    this.timeoutHandlers.splice(pos, 1);
                }
                emitter.emit('stop', {
                    name: 'stop',
                    data: undefined,
                });
            }, left);
        }
        return emitter.events('stop');
    }
}

export function stopTimeoutHandler(handler: TimeoutHandler) {
    if (handler.handler) {
        clearTimeout(handler.handler);
    }
}

export const makeTimeoutPromise = <T>(timeout: number, handler?: TimeoutHandler) => {
    return new Promise<T>((resolve, reject) => {
        if (timeout > 0) {
            if (handler) {
                handler.handler = setTimeout(() => {
                    reject(new TimeOutError());
                }, timeout);
            } else {
                setTimeout(() => {
                    reject(new TimeOutError());
                }, timeout);
            }
        } else if (timeout == 0) {
            reject(new TimeOutError());
        }
    });
}

export const makeTimeoutEvent = <ET extends TimeoutEmitEventMap>(emitter: Emittery<ET>, timeout: number): [AsyncIterableIterator<TimeoutEmitEventMap['timeout']>, () => void] => {
    const evts = emitter.events('timeout');
    const timeoutClear = timeoutEmit(evts, emitter, timeout);
    return [evts, () => {
        evts.return();
        timeoutClear();
    }];
}

export const timeoutEmit = <ET extends TimeoutEmitEventMap>(evts: AsyncIterableIterator<TimeoutEmitEventMap['timeout']>, emitter: Emittery<ET>, timeout: number) => {
    const t = setTimeout(() => {
        emitter.emit('timeout', {
            name: 'timeout',
            data: undefined,
        });
    }, timeout);
    return () => {
        clearTimeout(t);
    }
}