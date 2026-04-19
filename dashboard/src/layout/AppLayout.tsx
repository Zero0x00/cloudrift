import { NavLink, useLocation } from "react-router-dom";
import type { PropsWithChildren } from "react";
import { ThemeToggle } from "../components/ThemeToggle";
import { useScanContext } from "../hooks/useScanContext";

const staticNavItems = [
  { to: "/overview", label: "Overview", preserveSearch: false },
  { to: "/findings", label: "Findings", preserveSearch: true },
  { to: "/triage", label: "Triage", preserveSearch: true },
  { to: "/accounts", label: "Accounts", preserveSearch: false },
  { to: "/diff", label: "Diff", preserveSearch: false },
  { to: "/trust-report", label: "Trust Report", preserveSearch: false }
];

function navClassName(isActive: boolean): string {
  return [
    "rounded-md px-3 py-2 text-sm font-medium transition",
    isActive
      ? "bg-cyan-500/10 text-cyan-800 ring-1 ring-cyan-500/30 dark:bg-cyan-500/15 dark:text-cyan-200 dark:ring-cyan-400/40"
      : "text-slate-600 hover:bg-slate-200 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-white"
  ].join(" ");
}

export function AppLayout({ children }: PropsWithChildren) {
  const { selectedScanId, isResolvingScan } = useScanContext();
  const location = useLocation();

  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-200 bg-white/90 backdrop-blur dark:border-slate-800 dark:bg-slate-900/80">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4 px-6 py-4">
          <div className="flex min-w-[16rem] flex-wrap items-center gap-2">
            <span className="inline-block h-2.5 w-2.5 rounded-full bg-cyan-500 dark:bg-cyan-400" />
            <h1 className="text-sm font-semibold uppercase tracking-wide text-slate-800 dark:text-slate-100">
              Cloudrift
            </h1>
            <span className="ml-3 text-xs text-slate-500 dark:text-slate-400">
              {isResolvingScan ? "Resolving scan..." : `scan_id=${selectedScanId ?? "none"}`}
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <ThemeToggle />
            <nav className="flex flex-wrap items-center gap-2" aria-label="Primary">
              {staticNavItems.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.preserveSearch ? { pathname: item.to, search: location.search } : item.to}
                  className={({ isActive }) => navClassName(isActive)}
                >
                  {item.label}
                </NavLink>
              ))}
            </nav>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-7xl px-6 py-8">{children}</main>
    </div>
  );
}
