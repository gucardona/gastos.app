import { Navigate, Route, Routes } from "react-router";
import { Chat } from "./pages/Chat";
import { Division } from "./pages/Division";
import { Inbox } from "./pages/Inbox";
import { Login } from "./pages/Login";
import { Overview } from "./pages/Overview";
import { Settings } from "./pages/Settings";
import { Transactions } from "./pages/Transactions";
import { Shell } from "./Shell";

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<Shell />}>
        <Route index element={<Overview />} />
        <Route path="lancamentos" element={<Transactions />} />
        <Route path="recorrentes" element={<Navigate to="/lancamentos" replace />} />
        <Route path="divisao" element={<Division />} />
        <Route path="caixa" element={<Inbox />} />
        <Route path="chat" element={<Chat />} />
        <Route path="ajustes/:section?" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
