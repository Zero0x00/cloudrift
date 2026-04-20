import type {
  AccountsBreakdownResponse,
  DiffResponse,
  ExternalEntitiesQueryParams,
  ExternalEntitiesResponse,
  FindingDetailResponse,
  FindingsListResponse,
  FindingsQueryParams,
  RuntimeStatusResponse,
  ScanRunHistoryResponse,
  ScanListResponse,
  ScanRunStatusResponse,
  ScanStartRequest,
  ScanStartResponse,
  ScanSummaryResponse,
  ValidateProfileResponse
} from "./types";
import { ApiRequestError, parseAPIErrorBody } from "./httpError";

const API_BASE = "/api";

function makeQueryString(
  params:
    | Record<string, string | number | boolean | undefined>
    | FindingsQueryParams
    | ExternalEntitiesQueryParams
): string {
  const searchParams = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === "" || value === false) {
      continue;
    }
    searchParams.set(key, String(value));
  }
  const raw = searchParams.toString();
  return raw ? `?${raw}` : "";
}

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...(init?.headers ?? {})
    }
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
  getExternalEntities(
    scanId: string,
    params: ExternalEntitiesQueryParams = {}
  ): Promise<ExternalEntitiesResponse> {
    return fetchJSON<ExternalEntitiesResponse>(
      `/scans/${encodeURIComponent(scanId)}/external-entities${makeQueryString(params)}`
    );
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
  },
  getRuntimeStatus(): Promise<RuntimeStatusResponse> {
    return fetchJSON<RuntimeStatusResponse>("/runtime/status");
  },
  validateProfile(profile: string): Promise<ValidateProfileResponse> {
    return fetchJSON<ValidateProfileResponse>("/runtime/validate-profile", {
      method: "POST",
      body: JSON.stringify({ profile })
    });
  },
  startScan(req: ScanStartRequest): Promise<ScanStartResponse> {
    return fetchJSON<ScanStartResponse>("/scan/start", {
      method: "POST",
      body: JSON.stringify(req)
    });
  },
  getScanRunStatus(): Promise<ScanRunStatusResponse> {
    return fetchJSON<ScanRunStatusResponse>("/scan/status");
  },
  getScanRunHistory(): Promise<ScanRunHistoryResponse> {
    return fetchJSON<ScanRunHistoryResponse>("/scan/history");
  }
};
