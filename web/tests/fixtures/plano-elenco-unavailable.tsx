import React from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import PlanoElenco from "../../src/pages/PlanoElenco";

createRoot(document.getElementById("root")!).render(<BrowserRouter><PlanoElenco /></BrowserRouter>);
