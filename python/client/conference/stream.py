from aiortc import MediaStreamTrack

class LocalStream:
    tracks: list[MediaStreamTrack]
    labels: dict[str, str]