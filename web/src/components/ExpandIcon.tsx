export default function ExpandIcon({ expanded }: { expanded: boolean }) {
  return <svg className="expand-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false"><path d={expanded ? "m6 14 6-6 6 6" : "m6 10 6 6 6-6"} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" /></svg>;
}
