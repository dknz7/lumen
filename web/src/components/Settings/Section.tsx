import { JSX, ParentProps } from "solid-js";
import "./Section.css";

export interface SectionProps {
  title: string;
  description?: string;
  children?: JSX.Element;
}

export default function Section(props: ParentProps<SectionProps>) {
  return (
    <section class="settings-section">
      <header class="settings-section-header">
        <h2>{props.title}</h2>
        {props.description && <p>{props.description}</p>}
      </header>
      <div class="settings-section-body">{props.children}</div>
    </section>
  );
}
