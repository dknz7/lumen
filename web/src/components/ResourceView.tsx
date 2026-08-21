import { JSX, Match, Show, Switch } from "solid-js";
import type { Resource } from "solid-js";
import { errorMessage } from "./Toast";
import "./ResourceView.css";

/**
 * ResourceView renders a createResource safely.
 *
 * WHY THIS EXISTS — the single most important thing to know about Solid
 * resources: reading `data()` on a resource whose fetcher REJECTED re-throws
 * the error. Not undefined, not null: it throws, from inside whatever tracking
 * scope read it. `data.latest` throws too, including when a *refetch* fails
 * after a successful first load.
 *
 * So the idiomatic-looking `<Show when={items()}>` is a landmine. A Plex
 * hiccup, an expired token, or a laptop waking up and triggering a
 * refetch-on-focus doesn't degrade the page — it throws mid-flush and wedges
 * it. With no ErrorBoundary that took the whole app down.
 *
 * The rule this component enforces: check `.error` BEFORE touching the value.
 *
 * (Library.tsx had hand-written error UI and an auto-retry that were literally
 * unreachable, because the throw fired before either could run.)
 */
export interface ResourceViewProps<T> {
  resource: Resource<T>;
  /** Rendered while loading. A skeleton beats a spinner. */
  loading?: JSX.Element;
  /** Retry handler — usually the refetch from createResource. */
  onRetry?: () => void;
  /** Overrides the default error copy. */
  errorTitle?: string;
  /** Rendered when the resource resolves to nothing / an empty array. */
  empty?: JSX.Element;
  children: (value: T) => JSX.Element;
}

function isEmpty(v: unknown): boolean {
  return v === null || v === undefined || (Array.isArray(v) && v.length === 0);
}

export default function ResourceView<T>(props: ResourceViewProps<T>): JSX.Element {
  return (
    <Switch>
      {/* .error FIRST, always. */}
      <Match when={props.resource.error}>
        <div class="rv-error" role="alert">
          <p class="rv-error-title">{props.errorTitle ?? "Couldn't load this"}</p>
          <p class="rv-error-detail">{errorMessage(props.resource.error)}</p>
          <Show when={props.onRetry}>
            <button class="rv-retry" onClick={() => props.onRetry!()}>
              Try again
            </button>
          </Show>
        </div>
      </Match>

      <Match when={props.resource.loading}>
        {props.loading ?? <div class="rv-loading">Loading…</div>}
      </Match>

      <Match when={props.empty !== undefined && isEmpty(props.resource())}>
        {props.empty}
      </Match>

      <Match when={props.resource() !== undefined}>
        {props.children(props.resource() as T)}
      </Match>
    </Switch>
  );
}
