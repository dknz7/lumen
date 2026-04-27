import { createEffect, createMemo, createSignal, For, Index, JSX, onCleanup, Show } from "solid-js";
import { createSortable, useDragDropContext } from "@thisbeyond/solid-dnd";
import { ChevronDown, ChevronLeft, ChevronRight, GripVertical } from "./icons";
import { store as settingsStore } from "../state/settings";
import "./Shelf.css";

// Card-size base widths must mirror theme.css's --card-width-{s,m,l,xl}.
// Used to derive cardsPerRow for JS pagination.
const CARD_WIDTH_BASE_PX: Record<"s" | "m" | "l" | "xl", number> = {
  s: 120, m: 160, l: 200, xl: 240,
};

// Approximate horizontal grid gap — must match Shelf.css's `.shelf-page` gap.
const GRID_COL_GAP_PX = 16;

export interface ShelfProps {
  id: string;
  title: string;
  /**
   * When set together with items + renderItem, lays out cards in viewport-wide
   * pages of (cardsPerRow × rowsPerPage) cells, ROW-MAJOR fill within each
   * page (row 1 fills viewport-width left-to-right, then row 2). Floating
   * arrows scroll between pages. All Home shelves currently use 2.
   */
  rowsPerPage?: number;
  /** Card source array. Required for paginated mode. */
  items?: any[];
  /** Render fn for each item. Required when items is provided. */
  renderItem?: (item: any, index: number) => JSX.Element;
  /** Fallback content rendered when items isn't supplied (loading, empty,
   *  error stubs). Lays out in a single non-paginated grid. */
  children?: JSX.Element;
  /** Optional icon rendered between the collapse caret and the title. */
  icon?: JSX.Element;
  initialCollapsed?: boolean;
  /**
   * Set false when this shelf renders OUTSIDE a DragDropProvider/SortableProvider
   * (e.g. Continue Watching, which is pinned and never reorderable). Without
   * this escape hatch, createSortable null-derefs the missing context.
   */
  sortable?: boolean;
}

export default function Shelf(props: ShelfProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);

  const ctx = useDragDropContext();
  const canSortable = props.sortable !== false && ctx != null;
  const sortable = canSortable ? createSortable(props.id) : null;

  // Pagination state
  const [scrollerEl, setScrollerEl] = createSignal<HTMLDivElement | undefined>();
  const [scrollLeft, setScrollLeft] = createSignal(0);
  const [scrollMetrics, setScrollMetrics] = createSignal({ clientWidth: 0, scrollWidth: 0 });

  // Card width is derived from cardSize + zoom (both global settings). Computed
  // here directly rather than via getComputedStyle so it's reactive to settings
  // changes BEFORE applyRootDerived has touched the DOM.
  const cardWidthPx = createMemo(() => {
    const s = settingsStore.settings();
    if (!s) return CARD_WIDTH_BASE_PX.m;
    const base = CARD_WIDTH_BASE_PX[s.cardSize] ?? CARD_WIDTH_BASE_PX.m;
    return Math.round(base * (s.zoom / 100));
  });

  // Cards per row from current scroller width and card width.
  const cardsPerRow = createMemo(() => {
    const m = scrollMetrics();
    const cw = cardWidthPx();
    if (!m.clientWidth || !cw) return 1;
    return Math.max(1, Math.floor((m.clientWidth + GRID_COL_GAP_PX) / (cw + GRID_COL_GAP_PX)));
  });

  // Pages: chunks of (cardsPerRow × rowsPerPage). Each page = one viewport-width
  // row-major grid. Row 1 fills first, then row 2, then NEXT page on the right.
  const pages = createMemo(() => {
    const items = props.items;
    const rpp = props.rowsPerPage;
    if (!items || !rpp) return [] as any[][];
    const perPage = cardsPerRow() * rpp;
    if (perPage <= 0) return [items];
    const result: any[][] = [];
    for (let i = 0; i < items.length; i += perPage) {
      result.push(items.slice(i, i + perPage));
    }
    return result;
  });

  const isPaginated = () =>
    !!props.rowsPerPage &&
    !!props.items &&
    !!props.renderItem &&
    props.items.length > 0;

  const showLeft = () => isPaginated() && scrollLeft() > 4;
  const showRight = () => {
    if (!isPaginated()) return false;
    const m = scrollMetrics();
    return scrollLeft() + m.clientWidth < m.scrollWidth - 4;
  };

  function updateMetrics() {
    const el = scrollerEl();
    if (!el) return;
    setScrollLeft(el.scrollLeft);
    setScrollMetrics({
      clientWidth: el.clientWidth,
      scrollWidth: el.scrollWidth,
    });
  }

  function scrollByPage(direction: 1 | -1) {
    const el = scrollerEl();
    if (!el) return;
    el.scrollBy({ left: direction * el.clientWidth, behavior: "smooth" });
  }

  // Re-observe whenever the scroll container mounts. ResizeObserver covers
  // viewport resizes; MutationObserver covers async card mounts as Plex data
  // arrives (so right-arrow visibility settles once cards are in the DOM).
  createEffect(() => {
    const el = scrollerEl();
    if (!el || !isPaginated()) return;
    updateMetrics();
    const ro = new ResizeObserver(updateMetrics);
    ro.observe(el);
    const mo = new MutationObserver(updateMetrics);
    mo.observe(el, { childList: true, subtree: true });
    onCleanup(() => {
      ro.disconnect();
      mo.disconnect();
    });
  });

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
          <Show when={props.icon}>
            <span class="shelf-icon">{props.icon}</span>
          </Show>
          <h2 class="shelf-title">{props.title}</h2>
        </button>
        <Show when={sortable}>
          <span
            class="shelf-drag-handle"
            {...(sortable?.dragActivators ?? {})}
            title="Drag to reorder"
            aria-label="Drag handle"
          >
            <GripVertical size={20} />
          </span>
        </Show>
      </header>
      <Show when={!collapsed()}>
        <Show
          when={isPaginated()}
          fallback={<div class="shelf-cards">{props.children}</div>}
        >
          <div class="shelf-scroller">
            <div
              ref={setScrollerEl}
              class="shelf-cards paginated"
              onScroll={updateMetrics}
            >
              {/* <Index> for the outer pages iteration — keys by INDEX so
                  page-div slots stay mounted when pages() returns a new
                  outer array (which it does on every refetch since slice()
                  creates new arrays). The inner <For> still keys by item
                  reference, reusing tile DOM across data updates. Without
                  this, the outer For remounted page divs on every refetch,
                  destroying tile DOM mid-click → "double-click required"
                  bug (Session 6.5 round 2). */}
              <Index each={pages()}>
                {(pageItems) => (
                  <div
                    class="shelf-page"
                    style={{
                      "grid-template-columns": `repeat(${cardsPerRow()}, ${cardWidthPx()}px)`,
                      "grid-template-rows": `repeat(${props.rowsPerPage}, auto)`,
                    }}
                  >
                    <For each={pageItems()}>
                      {(item, idx) => props.renderItem!(item, idx())}
                    </For>
                  </div>
                )}
              </Index>
            </div>
            <Show when={showLeft()}>
              <button
                class="shelf-arrow left"
                aria-label="Scroll left"
                onClick={() => scrollByPage(-1)}
              >
                <ChevronLeft size={20} />
              </button>
            </Show>
            <Show when={showRight()}>
              <button
                class="shelf-arrow right"
                aria-label="Scroll right"
                onClick={() => scrollByPage(1)}
              >
                <ChevronRight size={20} />
              </button>
            </Show>
          </div>
        </Show>
      </Show>
    </section>
  );
}
