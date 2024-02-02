import logo from './logo.svg';
import './App.css';
import 'flexboxgrid';
import { useQueryParam, StringParam, NumberParam, withDefault } from 'use-query-params';
import { Video, VideoProps, useCreateOnce } from './Socket';
import { HlsPlayer } from './hls';

const ROW = 10;
const COL = 3;

const RowParam = withDefault(NumberParam, ROW)
const ColParam = withDefault(NumberParam, COL)
const ModParam = withDefault(StringParam, 'conference')

function App() {
  const [mode] = useQueryParam('mode', ModParam);
  let [row] = useQueryParam('row', RowParam);
  let [col] = useQueryParam('col', ColParam);
  let [ub] = useQueryParam("ub", withDefault(NumberParam, 0));
  let [cluster] = useQueryParam("cluster", withDefault(NumberParam, 0))
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
  if (mode !== 'conference' && mode !== 'hls') {
    return <div>The query param mode must be conference or hls.</div>
  }
  if (mode === 'hls') {
    row = col = 1;
  }
  if (12 % col != 0) {
    return <div>The query param col must be divisible by 12.</div>;
  }
  const colSize = 12 / col;
  for (let i = 0; i < row * col; ++i) {
    let signalPort = 8080;
    if (cluster > 1) {
      const offset = (i % col) % cluster;
      signalPort += offset;
    }
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
        uid: `${i + ub}`,
        uname: `user${i + ub}`,
        role: 'student',
        room: `room${i % row + ub}`,
      },
      publish: {
        labels: {
          uid: `${i + ub}`,
        },
      },
      subscribe: {
        labels: {
          uid: `${ub + ((i + row) % (row * col))}`,
        },
      },
      // signalHost: 'http://localhost:8080',
      // authHost: 'http://localhost:3100',
      signalHost: `http://192.168.1.233:${signalPort}`,
      authHost: 'http://192.168.1.233:3100',
    }
  }

  let Hls;
  if (mode === 'hls') {
    Hls = (
      <HlsPlayer indexUrl='http://192.168.1.233:12080/hls/0/master.m3u8' delay={15000}/>
    )
  } else {
    Hls = null;
  }
  return (
    <div className="App">
      <header className="App-header">
        <img src={logo} className="App-logo" alt="logo" />
        <div>
          { Hls }
          { [ ...Array(row).keys() ].map(r => (
            <div className='row' key={r}>
              { [ ...Array(col).keys() ].map(c => (
                <div className={`col-xs-${colSize}`} key={c}>
                  <Video {...props[c * row + r]}/>
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
