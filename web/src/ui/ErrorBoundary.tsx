import { Component, type ReactNode } from "react";
import { Link, useLocation } from "react-router";

class Boundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <div className="empty" role="alert">
        <p>Algo deu errado ao mostrar esta tela.</p>
        <button type="button" className="btn" onClick={() => this.setState({ failed: false })}>Tentar de novo</button>
        <Link className="link" to="/">Voltar ao início</Link>
      </div>
    );
  }
}

/** Catches a render error in a screen so the rest of the app stays usable; changing route starts over. */
export function ErrorBoundary({ children }: { children: ReactNode }) {
  return <Boundary key={useLocation().pathname}>{children}</Boundary>;
}
