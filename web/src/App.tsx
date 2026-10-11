import { Navigate, Route, Routes } from "react-router";
import { Login } from "./pages/Login";
import { Overview } from "./pages/Overview";
import { Settings } from "./pages/Settings";
import { Soon } from "./pages/Soon";
import { Transactions } from "./pages/Transactions";
import { Shell } from "./Shell";

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<Shell />}>
        <Route index element={<Overview />} />
        <Route path="lancamentos" element={<Transactions />} />
        <Route path="recorrentes" element={<Soon title="Recorrentes" />} />
        <Route path="divisao" element={<Soon title="Divisão" />} />
        <Route path="ajustes" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
