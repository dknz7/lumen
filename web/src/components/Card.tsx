import { A } from "@solidjs/router";
import { api } from "../api/client";
import "./Card.css";

export interface CardProps {
  title: string;
  year?: number;
  thumb?: string;       // server-relative path, e.g. /library/metadata/123/thumb/1234
  serverID: string;
  ratingKey: string;
  subtitle?: string;    // optional: for episodes, "Show Name — E03"
}

export default function Card(props: CardProps) {
  return (
    <A
      class="card"
      href={`/item/${props.serverID}/${props.ratingKey}`}
    >
      <div class="card-poster">
        {props.thumb ? (
          <img
            src={api.image(props.serverID, props.thumb)}
            alt={props.title}
            loading="lazy"
          />
        ) : (
          <div class="card-poster-placeholder">
            <span>{props.title.slice(0, 1)}</span>
          </div>
        )}
      </div>
      <div class="card-meta">
        <div class="card-title">{props.title}</div>
        {props.subtitle && <div class="card-subtitle">{props.subtitle}</div>}
        {props.year && <div class="card-year">{props.year}</div>}
      </div>
    </A>
  );
}
