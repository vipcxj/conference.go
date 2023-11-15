import urllib.request
import random
from conference.client import ConferenceClient

def requestToken(uid: str, uname: str, role: str, room: str):
    nonce = random.randint(0, 99999)
    request = urllib.request.Request(f'http://localhost:3100/token?uid={uid}&uname={uname}&role={role}&room={room}&nonce={nonce}')
    with urllib.request.urlopen(request) as response:
        return response.read().decode("utf-8")

async def main():
    pass

if __name__ == '__main__':
    main()