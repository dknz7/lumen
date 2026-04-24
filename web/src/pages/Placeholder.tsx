import { useLocation } from "@solidjs/router";

export default function Placeholder(props: { name: string; session: string }) {
  const loc = useLocation();
  return (
    <div style={{ "padding": "40px" }}>
      <h1 style={{ "color": "var(--text)" }}>{props.name}</h1>
      <p style={{ "color": "var(--text-muted)", "max-width": "60ch" }}>
        This page lands in <strong>{props.session}</strong>. Current route: <code>{loc.pathname}</code>.
      </p>
    </div>
  );
}
