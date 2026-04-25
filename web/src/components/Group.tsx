import { createSignal, JSX, Show } from "solid-js";
import { createSortable, useDragDropContext } from "@thisbeyond/solid-dnd";
import { ChevronDown, ChevronRight, GripVertical } from "./icons";
import "./Group.css";

export interface GroupProps {
  id: string;
  title: string;
  /** Optional icon rendered between the collapse caret and the title. */
  icon?: JSX.Element;
  initialCollapsed?: boolean;
  children?: JSX.Element;
}

export default function Group(props: GroupProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);

  // Only register with solid-dnd when a DragDropProvider is actually present.
  // Using `!= null` to catch both null AND undefined (useContext returns
  // undefined when there's no Provider — strict inequality let the null-deref
  // through, crashing the subtree with `Symbol.iterator on null`).
  const ctx = useDragDropContext();
  const canSortable = ctx != null;
  const sortable = canSortable ? createSortable(props.id) : null;

  return (
    <section
      ref={sortable?.ref}
      class="group"
      classList={{ "is-dragging": !!sortable?.isActiveDraggable }}
      style={
        sortable?.transform
          ? { transform: `translate(${sortable.transform.x}px, ${sortable.transform.y}px)` }
          : {}
      }
      data-group-id={props.id}
    >
      <header class="group-header-wrap">
        <button
          class="group-header"
          aria-expanded={!collapsed()}
          onClick={() => setCollapsed(!collapsed())}
        >
          <span class="caret">
            {collapsed() ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
          </span>
          <Show when={props.icon}>
            <span class="group-icon">{props.icon}</span>
          </Show>
          <h1 class="group-title">{props.title}</h1>
        </button>
        <Show when={sortable}>
          <span
            class="group-drag-handle"
            {...(sortable?.dragActivators ?? {})}
            title="Drag to reorder group"
          >
            <GripVertical size={22} />
          </span>
        </Show>
      </header>
      <Show when={!collapsed()}>
        <div class="group-body">{props.children}</div>
      </Show>
    </section>
  );
}
