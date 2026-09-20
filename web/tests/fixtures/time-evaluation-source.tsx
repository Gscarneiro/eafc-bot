import React from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import Time from "../../src/pages/Time";

createRoot(document.getElementById("root")!).render(<BrowserRouter><Time /></BrowserRouter>);
