import urllib.request
import random
import asyncio as aio
from conference.client import ConferenceClient, Configuration, Pattern

def requestToken(uid: str, uname: str, role: str, room: str):
    nonce = random.randint(0, 99999)
    request = urllib.request.Request(f'http://localhost:3100/token?uid={uid}&uname={uname}&role={role}&room={room}&nonce={nonce}')
    with urllib.request.urlopen(request) as response:
        return response.read().decode("utf-8")
    
ROW = 10
COL = 3

async def main():
    token = requestToken('999', 'ai', 'ai', '*')
    client = ConferenceClient(Configuration(
      signalUrl = 'http://192.168.1.233:8080/socket.io',
      token = token,
    ))
    rooms: list[str] = [f'room{r}' for r in range(COL)]
    await client.join(rooms)
    for ri, room in enumerate(rooms):
        uids = [ri + c * ROW for c in range(COL)]
        for uid in uids:
            st = await client.subscribe(Pattern.All(
                Pattern.TrackTypeIn('video'),
                Pattern.LabelsAllMatch({
                    'uid': f'{uid}',
                }),
            ))
            print(f'subscribed uid {uid} in room {room} with sub id {st.subId} and pub id {st.tracks[0].metadata.pubId}')

if __name__ == '__main__':
    aio.run(main())