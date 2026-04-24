import "./Skeleton.css";

export interface SkeletonProps {
  kind?: "card" | "line";
  count?: number;
}

export default function Skeleton(props: SkeletonProps) {
  const n = () => props.count ?? 1;
  const kind = () => props.kind ?? "line";
  return (
    <>
      {Array.from({ length: n() }).map(() => (
        <div class={`skeleton skeleton-${kind()}`} />
      ))}
    </>
  );
}
