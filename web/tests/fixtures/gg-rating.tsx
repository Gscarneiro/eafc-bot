import React from "react";
import { createRoot } from "react-dom/client";
import GGRating from "../../src/components/GGRating";

createRoot(document.getElementById("root")!).render(
  <>
    <div data-testid="different"><GGRating current={87.99} positional={97.73} positionalPosition="CM" /></div>
    <div data-testid="current-higher"><GGRating current={99} positional={97.9} positionalPosition="CB" /></div>
    <div data-testid="equal"><GGRating current={97.73} positional={97.729} positionalPosition="CM" /></div>
    <div data-testid="missing"><GGRating positional={97.73} positionalPosition="CM" /></div>
  </>,
);
