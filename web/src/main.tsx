import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import App from "./App";
import Status from "./pages/Status";
import Time from "./pages/Time";
import Gauntlet from "./pages/Gauntlet";
import PlanoElenco from "./pages/PlanoElenco";
import CardDetail from "./pages/CardDetail";
import Mercado from "./pages/Mercado";
import Evolucoes from "./pages/Evolucoes";
import Investimentos from "./pages/Investimentos";
import Configuracoes from "./pages/Configuracoes";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<App />}>
          <Route index element={<Status />} />
          <Route path="time" element={<Time />} />
          <Route path="time/gauntlet" element={<Gauntlet />} />
          <Route path="time/planos" element={<PlanoElenco />} />
          <Route path="time/:slug" element={<CardDetail />} />
          <Route path="mercado" element={<Mercado />} />
          <Route path="evolucoes" element={<Evolucoes />} />
          <Route path="capital/investimentos" element={<Investimentos section="investimentos" />} />
          <Route path="capital/vendas" element={<Investimentos section="vendas" />} />
          <Route path="capital/sbcs" element={<Investimentos section="sbcs" />} />
          <Route path="investimentos" element={<Navigate to="/capital/investimentos" replace />} />
          <Route path="configuracoes" element={<Configuracoes />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </React.StrictMode>,
);
