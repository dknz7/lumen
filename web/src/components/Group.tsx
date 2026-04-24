import { createSignal, JSX, Show } from "solid-js";
import "./Group.css";

export interface GroupProps {
  id: string;
  title: string;
  initialCollapsed?: boolean;
  children?: JSX.Element;
}

export default function Group(props: GroupProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);
  return (
    <section class="group" data-group-id={props.id}>
      <button
        class="group-header"
        aria-expanded={!collapsed()}
        onClick={() => setCollapsed(!collapsed())}
      >
        <span class="caret">{collapsed() ? "▸" : "▾"}</span>
        <h1 class="group-title">{props.title}</h1>
      </button>
      <Show when={!collapsed()}>
        <div class="group-body">{props.children}</div>
      </Show>
    </section>
  );
}
