import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route, Navigate, useParams } from "react-router-dom";
import App from "./App";
import Status from "./pages/Status";
import Time from "./pages/Time";
import ClubInsights from "./pages/ClubInsights";
import Gauntlet from "./pages/Gauntlet";
import PlanoElenco from "./pages/PlanoElenco";
import EditorElenco from "./pages/EditorElenco";
import CardDetail from "./pages/CardDetail";
import Mercado from "./pages/Mercado";
import PlanoMercado from "./pages/PlanoMercado";
import Agenda from "./pages/Agenda";
import Evolucoes from "./pages/Evolucoes";
import EvolucaoDetalhe from "./pages/EvolucaoDetalhe";
import AnaliseEvolucoes from "./pages/AnaliseEvolucoes";
import PathsSalvos from "./pages/PathsSalvos";
import Investimentos from "./pages/Investimentos";
import Configuracoes from "./pages/Configuracoes";
import FeedbackGameplay from "./pages/FeedbackGameplay";
import { inicializarTema } from "./theme";

inicializarTema();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<App />}>
          <Route index element={<Status />} />
          <Route path="time" element={<Time />} />
          <Route path="time/insights" element={<ClubInsights />} />
          <Route path="time/gauntlet" element={<Gauntlet />} />
		  <Route path="time/planos" element={<PlanoElenco />} />
		  <Route path="time/editor" element={<EditorElenco />} />
		  <Route path="time/:slug" element={<CardDetail />} />
          <Route path="mercado" element={<Mercado />} />
          <Route path="mercado/plano" element={<PlanoMercado />} />
          <Route path="agenda" element={<Agenda />} />
		  <Route path="feedback" element={<FeedbackGameplay />} />
          <Route path="evolucoes" element={<AnaliseEvolucoes />} />
          <Route path="evolucoes/catalogo" element={<Evolucoes />} />
          <Route path="evolucoes/catalogo/:slug" element={<EvolucaoDetalhe />} />
          <Route path="evolucoes/salvos" element={<PathsSalvos />} />
          <Route path="evolucoes/:slug" element={<EvolucaoLegadaRedirect />} />
          <Route path="capital" element={<Investimentos />} />
          <Route path="capital/investimentos" element={<Navigate to="/capital?tab=investimentos" replace />} />
          <Route path="capital/vendas" element={<Navigate to="/capital?tab=vendas" replace />} />
          <Route path="capital/sbcs" element={<Navigate to="/capital?tab=sbcs" replace />} />
          <Route path="investimentos" element={<Navigate to="/capital" replace />} />
          <Route path="configuracoes" element={<Configuracoes />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  </React.StrictMode>,
);

function EvolucaoLegadaRedirect() {
  const { slug } = useParams();
  return <Navigate to={`/evolucoes/catalogo/${encodeURIComponent(slug ?? "")}`} replace />;
}
