import { createSignal, JSX, Show } from "solid-js";
import { createSortable } from "@thisbeyond/solid-dnd";
import { ChevronDown, ChevronRight, GripVertical } from "./icons";
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
  const sortable = createSortable(props.id);

  return (
    <section
      ref={sortable.ref}
      class="shelf"
      classList={{ "is-dragging": sortable.isActiveDraggable }}
      style={sortable.transform ? { transform: `translate(${sortable.transform.x}px, ${sortable.transform.y}px)` } : {}}
      data-shelf-id={props.id}
    >
      <header class="shelf-header">
        <button
          class="shelf-collapse-btn"
          aria-expanded={!collapsed()}
          onClick={() => setCollapsed(!collapsed())}
        >
          <span class="caret">
            {collapsed() ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
          </span>
          <h2 class="shelf-title">{props.title}</h2>
        </button>
        <span class="shelf-drag-handle" {...sortable.dragActivators} title="Drag to reorder" aria-label="Drag handle">
          <GripVertical size={16} />
        </span>
      </header>
      <Show when={!collapsed()}>
        <div class="shelf-cards" style={{ "--rows-per-shelf": rowsPerShelf() }}>
          {props.children}
        </div>
      </Show>
    </section>
  );
}
