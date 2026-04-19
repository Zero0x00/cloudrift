import { Navigate, Route, Routes } from "react-router-dom";
import { AppLayout } from "./layout/AppLayout";
import { AccountsPage } from "./pages/Accounts";
import { DiffPage } from "./pages/Diff";
import { FindingsPage } from "./pages/Findings";
import { OverviewPage } from "./pages/Overview";
import { ScanControlCenterPage } from "./pages/ScanControlCenter";
import { TrustReportPage } from "./pages/TrustReport";

export default function App() {
  return (
    <AppLayout>
      <Routes>
        <Route path="/" element={<Navigate to="/overview" replace />} />
        <Route path="/overview" element={<OverviewPage />} />
        <Route path="/scan-control" element={<ScanControlCenterPage />} />
        <Route path="/findings" element={<FindingsPage />} />
        <Route path="/triage" element={<FindingsPage triage />} />
        <Route path="/accounts" element={<AccountsPage />} />
        <Route path="/diff" element={<DiffPage />} />
        <Route path="/trust-report" element={<TrustReportPage />} />
      </Routes>
    </AppLayout>
  );
}
