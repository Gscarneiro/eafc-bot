import React from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import TimePlanejador from "../../src/pages/TimePlanejador";

createRoot(document.getElementById("root")!).render(<BrowserRouter><TimePlanejador regua={0} /></BrowserRouter>);
