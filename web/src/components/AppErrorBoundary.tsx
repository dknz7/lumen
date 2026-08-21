import { ErrorBoundary, ParentProps } from "solid-js";
import { errorMessage } from "./Toast";
import "./AppErrorBoundary.css";

// The last line of defence.
//
// Solid re-throws when you read an errored resource, so a single unguarded
// `items()` anywhere could take the entire app down to a white screen with the
// error only visible in a console nobody has open — and in a WebView2 window
// there is no devtools tab to check.
//
// Individual components should use ResourceView and never reach this. When
// they do reach it, the user gets something they can act on instead of nothing.
export default function AppErrorBoundary(props: ParentProps) {
  return (
    <ErrorBoundary
      fallback={(err, reset) => (
        <div class="app-error" role="alert">
          <h1 class="app-error-title">Something broke</h1>
          <p class="app-error-detail">{errorMessage(err, "An unexpected error occurred.")}</p>
          <div class="app-error-actions">
            <button class="app-error-btn app-error-btn--primary" onClick={reset}>
              Try again
            </button>
            <button class="app-error-btn" onClick={() => window.location.assign("/")}>
              Back to Home
            </button>
          </div>
          <p class="app-error-hint">
            If this keeps happening, Settings &rsaquo; About has a "Copy
            diagnostics" button and a link to report it.
          </p>
        </div>
      )}
    >
      {props.children}
    </ErrorBoundary>
  );
}
