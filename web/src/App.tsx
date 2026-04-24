import { ParentProps } from "solid-js";

// Task 13/14 populate this with TopBar + LeftMenu + content area.
export default function App(props: ParentProps) {
  return (
    <div style={{ "padding": "20px" }}>
      <h1>Lumen</h1>
      {props.children}
    </div>
  );
}
