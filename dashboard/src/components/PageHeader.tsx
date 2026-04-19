interface PageHeaderProps {
  title: string;
  description: string;
  scanId: string | null;
}

export function PageHeader({ title, description, scanId }: PageHeaderProps) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h2 className="text-2xl font-semibold text-slate-900 dark:text-slate-100">{title}</h2>
        <p className="mt-1 text-sm text-slate-600 dark:text-slate-400">{description}</p>
      </div>
      <code className="rounded-md border border-slate-300 bg-white px-3 py-1.5 text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300">
        scan_id={scanId ?? "none"}
      </code>
    </div>
  );
}
