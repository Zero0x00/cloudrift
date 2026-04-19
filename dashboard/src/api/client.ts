import type {
  AccountsBreakdownResponse,
  DiffResponse,
  FindingDetailResponse,
  FindingsListResponse,
  FindingsQueryParams,
  ScanListResponse,
  ScanSummaryResponse
} from "./types";
import { ApiRequestError, parseAPIErrorBody } from "./httpError";

const API_BASE = "/api";

function makeQueryString(
  params: Record<string, string | number | undefined> | FindingsQueryParams
): string {
  const searchParams = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "") {
      continue;
    }
    searchParams.set(key, String(value));
  }
  const raw = searchParams.toString();
  return raw ? `?${raw}` : "";
}

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: { Accept: "application/json" }
  });

  if (!response.ok) {
    let parsed = null as ReturnType<typeof parseAPIErrorBody>;
    try {
      parsed = parseAPIErrorBody(await response.json());
    } catch {
      // ignore body parse errors
    }
    const msg = parsed?.error ?? `${response.status} ${response.statusText || "Error"}`;
    throw new ApiRequestError(msg, response.status, parsed?.code, parsed?.details);
  }

  return (await response.json()) as T;
}

export const apiClient = {
  getScans(): Promise<ScanListResponse> {
    return fetchJSON<ScanListResponse>("/scans");
  },
  getSummary(scanId: string): Promise<ScanSummaryResponse> {
    return fetchJSON<ScanSummaryResponse>(`/scans/${encodeURIComponent(scanId)}/summary`);
  },
  getFindings(scanId: string, params: FindingsQueryParams = {}): Promise<FindingsListResponse> {
    return fetchJSON<FindingsListResponse>(
      `/scans/${encodeURIComponent(scanId)}/findings${makeQueryString(params)}`
    );
  },
  getFindingDetail(scanId: string, findingId: string): Promise<FindingDetailResponse> {
    return fetchJSON<FindingDetailResponse>(
      `/scans/${encodeURIComponent(scanId)}/findings/${encodeURIComponent(findingId)}`
    );
  },
  getAccounts(scanId: string): Promise<AccountsBreakdownResponse> {
    return fetchJSON<AccountsBreakdownResponse>(`/scans/${encodeURIComponent(scanId)}/accounts`);
  },
  getDiff(oldScanId: string, newScanId: string): Promise<DiffResponse> {
    return fetchJSON<DiffResponse>(
      `/diff${makeQueryString({ old: oldScanId, new: newScanId })}`
    );
  }
};
