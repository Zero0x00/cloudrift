import { StatePanel } from "./StatePanel";

export function ScanRequired() {
  return (
    <StatePanel>
      No scan selected yet. Provide a <code>scan_id</code> query parameter or run a scan so one can be auto-selected.
    </StatePanel>
  );
}
