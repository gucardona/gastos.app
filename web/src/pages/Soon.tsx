import { PageBar } from "../components/PageBar";

export function Soon({ title }: { title: string }) {
  return (
    <>
      <PageBar />
      <h1 style={{ font: "700 28px var(--display)", margin: "8px 0" }}>{title}</h1>
      <p className="muted">Esta tela chega na próxima etapa da reescrita.</p>
    </>
  );
}
