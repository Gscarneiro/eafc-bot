import React from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import CardDetail from "../../src/pages/CardDetail";
import "../../src/theme.css";

createRoot(document.getElementById("root")!).render(
  <MemoryRouter initialEntries={["/time/dybala"]}>
    <Routes><Route path="/time/:slug" element={<CardDetail/>}/></Routes>
  </MemoryRouter>,
);
