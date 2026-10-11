import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App.jsx";
import "./style.css";
import UIProvider from "./worldbuilding/ui/UIProvider.jsx";
import "./worldbuilding/ui.css";
createRoot(document.getElementById("root")).render(
  <UIProvider>
    <App />
  </UIProvider>,
);
