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
   * When true (default), the shelf participates in the parent SortableProvider
   * and shows a drag handle on hover. Pass `false` for shelves that render
   * OUTSIDE a DragDropProvider (e.g. Continue Watching, which is pinned) —
   * calling createSortable there would null-deref the missing context.
   */
  sortable?: boolean;
}

export default function Shelf(props: ShelfProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);
  const rowsPerShelf = () => props.rowsPerShelf ?? 3;

  // Only register with solid-dnd when BOTH the prop allows it AND a
  // DragDropProvider is actually in the ancestor tree. The context check is
  // the defensive belt-and-braces — without it, an accidental use outside a
  // provider crashes the whole subtree with a cryptic Symbol.iterator error.
  const canSortable = props.sortable !== false && useDragDropContext() !== null;
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
