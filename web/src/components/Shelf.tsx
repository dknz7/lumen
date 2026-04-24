import { createSignal, JSX, Show } from "solid-js";
import { createSortable, useDragDropContext } from "@thisbeyond/solid-dnd";
import { ChevronDown, ChevronRight, GripVertical } from "./icons";
import "./Shelf.css";

export interface ShelfProps {
  id: string;
  title: string;
  rowsPerShelf?: number; // default 3
  children?: JSX.Element;
  initialCollapsed?: boolean;
  /**
   * Set false when this shelf renders OUTSIDE a DragDropProvider/SortableProvider
   * (e.g. Continue Watching, which is pinned and never reorderable). Without
   * this escape hatch, createSortable null-derefs the missing context and
   * crashes the subtree with `Symbol.iterator on null`.
   */
  sortable?: boolean;
}

export default function Shelf(props: ShelfProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);
  const rowsPerShelf = () => props.rowsPerShelf ?? 3;

  // Register with solid-dnd only when BOTH the prop allows it AND a
  // DragDropProvider is actually in the ancestor tree. Using loose `!= null`
  // catches both null and undefined since useContext returns undefined when
  // no Provider exists.
  const ctx = useDragDropContext();
  const canSortable = props.sortable !== false && ctx != null;
  const sortable = canSortable ? createSortable(props.id) : null;

  return (
    <section
      ref={sortable?.ref}
      class="shelf"
      classList={{ "is-dragging": !!sortable?.isActiveDraggable }}
      style={
        sortable?.transform
          ? { transform: `translate(${sortable.transform.x}px, ${sortable.transform.y}px)` }
          : {}
      }
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
        <Show when={sortable}>
          <span
            class="shelf-drag-handle"
            {...(sortable?.dragActivators ?? {})}
            title="Drag to reorder"
            aria-label="Drag handle"
          >
            <GripVertical size={16} />
          </span>
        </Show>
      </header>
      <Show when={!collapsed()}>
        <div class="shelf-cards" style={{ "--rows-per-shelf": rowsPerShelf() }}>
          {props.children}
        </div>
      </Show>
    </section>
  );
}
