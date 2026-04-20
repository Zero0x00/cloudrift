import type { ComponentType, PropsWithChildren, SVGProps } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { CurrentScanCard } from "../components/CurrentScanCard";
import { ThemeToggle } from "../components/ThemeToggle";
import {
  IconGitDiff,
  IconGlobe,
  IconLayoutDashboard,
  IconList,
  IconScan,
  IconShield
} from "../lib/icons";
import { useScanContext } from "../hooks/useScanContext";

const staticNavItems: {
  to: string;
  label: string;
  preserveSearch: boolean;
  Icon: ComponentType<SVGProps<SVGSVGElement>>;
}[] = [
  { to: "/overview", label: "Dashboard", preserveSearch: false, Icon: IconLayoutDashboard },
  { to: "/triage", label: "Triage", preserveSearch: true, Icon: IconList },
  { to: "/scan-control", label: "Scan Control", preserveSearch: false, Icon: IconScan },
  { to: "/findings", label: "Findings", preserveSearch: true, Icon: IconList },
  { to: "/trust-report", label: "Access", preserveSearch: false, Icon: IconShield },
  { to: "/external-entities", label: "External entities", preserveSearch: false, Icon: IconGlobe },
  { to: "/diff", label: "Changes", preserveSearch: false, Icon: IconGitDiff }
];

function navClassName(isActive: boolean): string {
  return [
    "flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition",
    isActive
      ? "bg-cyan-500/10 text-cyan-800 ring-1 ring-cyan-500/30 dark:bg-cyan-500/15 dark:text-cyan-200 dark:ring-cyan-400/40"
      : "text-slate-600 hover:bg-slate-200 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-white"
  ].join(" ");
}

export function AppLayout({ children }: PropsWithChildren) {
  const location = useLocation();
  const currentSearchParams = new URLSearchParams(location.search);
  const scanIdParam = currentSearchParams.get("scan_id");
  const currentViewParam = currentSearchParams.get("view");
  const isOnDashboard = location.pathname === "/overview";

  const buildNavSearch = (item: (typeof staticNavItems)[number]): string => {
    if (item.preserveSearch) {
      return location.search;
    }

    const next = new URLSearchParams();
    if (scanIdParam) {
      next.set("scan_id", scanIdParam);
    }

    // Preserve dashboard mode only when already navigating within dashboard context.
    if (item.to === "/overview" && isOnDashboard && currentViewParam) {
      next.set("view", currentViewParam);
    }

    const search = next.toString();
    return search ? `?${search}` : "";
  };

  return (
    <div className="flex min-h-screen flex-col md:flex-row">
      <aside className="flex w-full shrink-0 flex-col border-b border-slate-200 bg-white/95 dark:border-slate-800 dark:bg-slate-900/95 md:w-60 md:border-b-0 md:border-r">
        <div className="flex items-center gap-2 border-b border-slate-200 px-4 py-3 dark:border-slate-800">
          <span className="inline-block h-2.5 w-2.5 shrink-0 rounded-full bg-cyan-500 dark:bg-cyan-400" />
          <h1 className="text-sm font-semibold uppercase tracking-wide text-slate-800 dark:text-slate-100">
            Cloudrift
          </h1>
        </div>

        <div className="px-3 py-3">
          <CurrentScanCard />
        </div>

        <nav className="flex flex-1 flex-col gap-0.5 px-2 pb-2" aria-label="Primary">
          {staticNavItems.map((item) => (
            <NavLink
              key={item.to}
              to={{ pathname: item.to, search: buildNavSearch(item) }}
              className={({ isActive }) => navClassName(isActive)}
            >
              <item.Icon className="shrink-0 opacity-80" />
              {item.label}
            </NavLink>
          ))}
        </nav>

        <div className="mt-auto border-t border-slate-200 px-3 py-3 dark:border-slate-800">
          <div className="flex items-center justify-between gap-2">
            <span className="cr-helper">Theme</span>
            <ThemeToggle />
          </div>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col bg-slate-50 dark:bg-slate-950">
        <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6 sm:py-8">{children}</main>
      </div>
    </div>
  );
}
