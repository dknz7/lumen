import { createSignal, JSX, Show } from "solid-js";
import { createSortable } from "@thisbeyond/solid-dnd";
import { ChevronDown, ChevronRight, GripVertical } from "./icons";
import "./Group.css";

export interface GroupProps {
  id: string;
  title: string;
  initialCollapsed?: boolean;
  children?: JSX.Element;
}

export default function Group(props: GroupProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);
  const sortable = createSortable(props.id);
  return (
    <section
      ref={sortable.ref}
      class="group"
      classList={{ "is-dragging": sortable.isActiveDraggable }}
      style={sortable.transform ? { transform: `translate(${sortable.transform.x}px, ${sortable.transform.y}px)` } : {}}
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
          <h1 class="group-title">{props.title}</h1>
        </button>
        <span class="group-drag-handle" {...sortable.dragActivators} title="Drag to reorder group">
          <GripVertical size={16} />
        </span>
      </header>
      <Show when={!collapsed()}>
        <div class="group-body">{props.children}</div>
      </Show>
    </section>
  );
}
