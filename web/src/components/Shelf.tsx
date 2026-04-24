import { createSignal, JSX, Show } from "solid-js";
import "./Shelf.css";

export interface ShelfProps {
  id: string;
  title: string;
  rowsPerShelf?: number; // default 3
  children?: JSX.Element; // cards
  initialCollapsed?: boolean;
}

export default function Shelf(props: ShelfProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);
  const rowsPerShelf = () => props.rowsPerShelf ?? 3;

  return (
    <section class="shelf" data-shelf-id={props.id}>
      <header class="shelf-header">
        <button
          class="shelf-collapse-btn"
          aria-expanded={!collapsed()}
          onClick={() => setCollapsed(!collapsed())}
        >
          <span class="caret">{collapsed() ? "▸" : "▾"}</span>
          <h2 class="shelf-title">{props.title}</h2>
        </button>
      </header>
      <Show when={!collapsed()}>
        <div
          class="shelf-cards"
          style={{ "--rows-per-shelf": rowsPerShelf() }}
        >
          {props.children}
        </div>
      </Show>
    </section>
  );
}
