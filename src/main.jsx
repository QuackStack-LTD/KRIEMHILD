import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App.jsx";
import AuthProvider from "./auth/AuthProvider.jsx";
import { installAuthenticatedFetch } from "./auth/api.js";
import "./style.css";
import UIProvider from "./worldbuilding/ui/UIProvider.jsx";
import "./worldbuilding/ui.css";
installAuthenticatedFetch();
createRoot(document.getElementById("root")).render(
  <UIProvider>
    <AuthProvider>
      <App />
    </AuthProvider>
  </UIProvider>,
);
