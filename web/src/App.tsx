import { Navigate, Route, Routes } from "react-router";
import { Division } from "./pages/Division";
import { Inbox } from "./pages/Inbox";
import { Login } from "./pages/Login";
import { Overview } from "./pages/Overview";
import { Settings } from "./pages/Settings";
import { Recurring } from "./pages/Recurring";
import { Transactions } from "./pages/Transactions";
import { Shell } from "./Shell";

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<Shell />}>
        <Route index element={<Overview />} />
        <Route path="lancamentos" element={<Transactions />} />
        <Route path="recorrentes" element={<Recurring />} />
        <Route path="divisao" element={<Division />} />
        <Route path="caixa" element={<Inbox />} />
        <Route path="ajustes" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
