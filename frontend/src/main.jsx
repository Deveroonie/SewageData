import { BrowserRouter, Route, Routes, useLocation } from "react-router";
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.jsx'
import * as Sentry from "@sentry/react";
import Historical from "./pages/Historical.jsx";
import { useEffect } from "react";


Sentry.init({
  dsn: "https://8a1d5a8f5df1a1e7f823ae60f1280467@o4505861703729152.ingest.us.sentry.io/4511101740187648",
  sendDefaultPii: false,
  integrations: [
    Sentry.replayIntegration()
  ],
  // Session Replay
  replaysSessionSampleRate: 0.1,
  replaysOnErrorSampleRate: 1.0,
  enableLogs: true
});

const container = document.getElementById("root");
createRoot(container).render(
    <BrowserRouter>
      <PageTracker />
      <Routes>
          <Route path="/" element={<App />} />
          <Route path="/historical" element={<Historical />} />
      </Routes>
    </BrowserRouter>,
)

function PageTracker() {
  const location = useLocation();
  useEffect(() => {
    if (window.umami) {
      window.umami.track((props) => ({ ...props, url: location.pathname + location.search }));
    }
  }, [location]);
  return null;
}