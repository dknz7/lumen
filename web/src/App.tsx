import { ParentProps } from "solid-js";
import TopBar from "./components/TopBar";
import LeftMenu from "./components/LeftMenu";

export default function App(props: ParentProps) {
  return (
    <div class="app-shell">
      <TopBar />
      <div class="app-body">
        <LeftMenu />
        <main class="content">
          {props.children}
        </main>
      </div>
    </div>
  );
}
