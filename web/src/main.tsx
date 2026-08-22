import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import App from "./App";
import Status from "./pages/Status";
import Time from "./pages/Time";
import CardDetail from "./pages/CardDetail";
import Mercado from "./pages/Mercado";
import Evolucoes from "./pages/Evolucoes";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<App />}>
          <Route index element={<Status />} />
          <Route path="time" element={<Time />} />
          <Route path="time/:slug" element={<CardDetail />} />
          <Route path="mercado" element={<Mercado />} />
          <Route path="evolucoes" element={<Evolucoes />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </React.StrictMode>,
);
