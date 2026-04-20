import type { ReactNode } from "react";
import { EXTERNAL_LINK_REL, safeExternalUrl } from "../lib/urlSafety";

type SafeExternalLinkProps = {
  href: string;
  children: ReactNode;
  className?: string;
  fallback?: ReactNode;
};

/**
 * Renders a hardened external link only when URL passes sanitization.
 * Falls back to plain content when URL is unsafe or invalid.
 */
export function SafeExternalLink({ href, children, className, fallback }: SafeExternalLinkProps) {
  const safeHref = safeExternalUrl(href);
  if (!safeHref) {
    return <>{fallback ?? children}</>;
  }

  return (
    <a href={safeHref} target="_blank" rel={EXTERNAL_LINK_REL} className={className}>
      {children}
    </a>
  );
}
