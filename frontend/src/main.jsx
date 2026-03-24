import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.jsx'
import * as Sentry from "@sentry/react";


Sentry.init({
  dsn: "https://8a1d5a8f5df1a1e7f823ae60f1280467@o4505861703729152.ingest.us.sentry.io/4511101740187648",
  sendDefaultPii: true,
  integrations: [
    Sentry.replayIntegration()
  ],
  // Session Replay
  replaysSessionSampleRate: 0.1,
  replaysOnErrorSampleRate: 1.0,
  enableLogs: true
});

const container = document.getElementById("root");
const root = createRoot(container);
root.render(<App />);