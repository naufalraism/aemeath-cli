import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// While developing, this app runs on http://localhost:5173 and forwards the API
// calls to the Go server (started with "aemeath dev").
const api = "http://localhost:3000";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": api,
      "/docs": api,
      "/openapi.json": api,
    },
  },
});
