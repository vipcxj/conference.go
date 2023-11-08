import React from 'react';
import logo from './logo.svg';
import './App.css';

import { testSocket, useDelay } from './Socket'

function App() {
  const inputVideoRef = React.useRef<HTMLVideoElement>(null);
  const outputVideoRef0 = React.useRef<HTMLVideoElement>(null);
  const outputVideoRef1 = React.useRef<HTMLVideoElement>(null);
  const outputVideoRefs = [outputVideoRef0, outputVideoRef1]
  useDelay(testSocket, inputVideoRef, outputVideoRefs);
  return (
    <div className="App">
      <header className="App-header">
        <img src={logo} className="App-logo" alt="logo" />
        <video ref={inputVideoRef} autoPlay controls/>
        <video ref={outputVideoRefs[0]} autoPlay controls/>
        <video ref={outputVideoRefs[1]} autoPlay controls/>
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
