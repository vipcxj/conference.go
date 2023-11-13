from dataclasses import dataclass

@dataclass
class Track:
    type: str
    pubId: str
    globalId: str
    bindId: str
    rid: str
    streamId: str
    labels: dict[str, str] | None