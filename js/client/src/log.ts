import log, { Logger, LogLevelNames, LogLevelNumbers, LoggingMethod } from 'loglevel';

function applyPrefix(logger: Logger): Logger {
    const tag = `with-prefix`;
    const ofactory = logger.methodFactory;
    if ((ofactory as any).tag === tag) {
        return logger;
    }
    logger.methodFactory = function(methodName: LogLevelNames, level: LogLevelNumbers, loggerName: string | symbol): LoggingMethod {
        const method = ofactory(methodName, level, loggerName);
        return (message: string) => {
            return method(`[${String(loggerName)}] ${message}`);
        }
    };
    (logger.methodFactory as any).tag = tag;
    return logger;
}

applyPrefix(log)
log.setLevel('TRACE', false);

export function getLogger(name: string) {
    const logger = log.getLogger(name);
    applyPrefix(logger);
    return logger;
}
