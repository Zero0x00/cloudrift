import type { ReactNode } from "react";

export function StatePanel({
  children,
  intent = "default",
  title
}: {
  children: ReactNode;
  intent?: "default" | "error" | "empty";
  /** Short headline (e.g. error category); body stays in children. */
  title?: string;
}) {
  const classes =
    intent === "error"
      ? "border-rose-300 bg-rose-50 text-rose-900 dark:border-rose-900/70 dark:bg-rose-950/30 dark:text-rose-200"
      : intent === "empty"
        ? "border-slate-200 bg-slate-50 text-slate-600 dark:border-slate-800/80 dark:bg-slate-900/60 dark:text-slate-400"
        : "border-slate-200 bg-white text-slate-700 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300";
  return (
    <div className={`hs-card p-6 text-sm ${classes}`}>
      {title ? <p className="hs-state-title">{title}</p> : null}
      <div className={title ? "text-[0.9375rem] leading-relaxed" : undefined}>{children}</div>
    </div>
  );
}
