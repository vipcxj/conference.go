from collections.abc import Mapping, MutableMapping
from typing import Any
import os
import logging

def getLogger(name: str | None = None) -> logging.Logger:
    return logging.getLogger(name)

def getLevel(name: str | None) -> str | int | None:
    level = None
    if name:
        level = os.environ.get(f'LOGGER_{name.upper().replace("-", "_")}_LEVEL')
    if not level:
        level = os.environ.get(f'LOGGER_OWT_LEVEL', default=os.environ.get(f'LOGGER_LEVEL'))
    if level:
        if level.isdigit():
            level = int(level)
        else:
            level = level.upper()
    return level
        

def configLogger(logger: logging.Logger) -> logging.Logger:
    level = getLevel(logger.name)
    if level is not None:
        logger.setLevel(level)
    return logger
    

class MyLoggerAdapter(logging.LoggerAdapter):
    
    prefix: str
    
    def __init__(self, logger: logging.Logger, prefix: str | None = None, extra: Mapping[str, object] | None = None) -> None:
        super().__init__(logger, extra)
        self.prefix = prefix or ''
        
    def process(self, msg: Any, kwargs: MutableMapping[str, Any]) -> tuple[Any, MutableMapping[str, Any]]:
        if 'extra' not in kwargs:
            kwargs["extra"] = self.extra

        return f'{self.prefix}{msg}', kwargs