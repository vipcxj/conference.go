import logo from './logo.svg';
import './App.css';
import 'flexboxgrid'
import { Video, VideoProps, useCreateOnce } from './Socket'

const ROW = 10;
const COL = 3;
const COL_SIZE = 12 / COL;

function App() {
  const props: Array<VideoProps> = [];
  const stream = useCreateOnce(async () => {
    const stream = await navigator.mediaDevices.getUserMedia({
      video: {
        width: 320,
        height: 240,
        frameRate: 6,
      },
    });
    return stream;
  });
  for (let i = 0; i < ROW * COL; ++i) {
    props[i] = {
      name: `client${i}`,
      stream: stream,
      rtcConfig: {
        iceServers: [{
          urls: 'turn:192.168.1.233:5349',
          username: 'admin',
          credential: '123456',
        }]
      },
      auth: {
        uid: `${i}`,
        uname: `user${i}`,
        role: 'student',
        room: `room${i % ROW}`,
      },
      publish: {
        labels: {
          uid: `${i}`,
        },
      },
      subscribe: {
        labels: {
          uid: `${(i + ROW) % (ROW * COL)}`,
        },
      },
      signalHost: 'http://192.168.1.233:8080',
      authHost: 'http://192.168.1.233:3100',
    }
  }
  return (
    <div className="App">
      <header className="App-header">
        <img src={logo} className="App-logo" alt="logo" />
        <div>
          { [ ...Array(ROW).keys() ].map(r => (
            <div className='row' key={r}>
              { [ ...Array(COL).keys() ].map(c => (
                <div className={`col-xs-${COL_SIZE}`} key={c}>
                  <Video {...props[c * ROW + r]}/>
                </div>
              ))}
            </div>
          ))}
        </div>
        <p>
          Edit <code>src/App.tsx</code> and save to reload.
        </p>
        <a
          className="App-link"
          href="https://reactjs.org"
          target="_blank"
          rel="noopener noreferrer"
        >
          Learn React
        </a>
      </header>
    </div>
  );
}

export default App;
