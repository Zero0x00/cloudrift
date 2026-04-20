const SAFE_EXTERNAL_PROTOCOLS = new Set(["http:", "https:"]);

export const EXTERNAL_LINK_REL = "noopener noreferrer";

/**
 * Returns a sanitized absolute external URL, or null if unsafe/invalid.
 *
 * Rules:
 * - allow only http/https protocols
 * - reject credentials in URL
 * - reject empty hosts
 * - normalize host-only inputs (e.g. "example.com") to https
 */
export function safeExternalUrl(raw: string): string | null {
  const value = raw.trim();
  if (!value) {
    return null;
  }
  if (/^[a-zA-Z][a-zA-Z\d+\-.]*:\/\/\//.test(value)) {
    return null;
  }

  const parsed = parseExternalUrl(value);
  if (!parsed) {
    return null;
  }
  if (!SAFE_EXTERNAL_PROTOCOLS.has(parsed.protocol)) {
    return null;
  }
  if (!parsed.hostname) {
    return null;
  }
  if (parsed.username || parsed.password) {
    return null;
  }

  return parsed.toString();
}

function parseExternalUrl(value: string): URL | null {
  try {
    return new URL(value);
  } catch {
    // If input already looks like a URI scheme prefix, treat parse failure as invalid.
    if (/^[a-zA-Z][a-zA-Z\d+\-.]*:/.test(value)) {
      return null;
    }
    try {
      return new URL(`https://${value}`);
    } catch {
      return null;
    }
  }
}
