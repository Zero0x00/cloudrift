interface PageHeaderProps {
  title: string;
  description: string;
}

export function PageHeader({ title, description }: PageHeaderProps) {
  return (
    <div className="mb-6">
      <h2 className="cr-page-title">{title}</h2>
      <p className="cr-body mt-2 max-w-3xl text-slate-600 dark:text-slate-400">{description}</p>
    </div>
  );
}
