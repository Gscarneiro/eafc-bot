import { useEffect, useState } from "react";

export default function SearchInput({ value, onChange, placeholder = "buscar" }: { value: string; onChange: (value: string) => void; placeholder?: string }) {
  const [draft, setDraft] = useState(value);
  useEffect(() => setDraft(value), [value]);
  useEffect(() => {
    const timer = window.setTimeout(() => { if (draft !== value) onChange(draft); }, 300);
    return () => window.clearTimeout(timer);
  }, [draft, onChange, value]);
  return <label className="control-search"><span className="sr-only">{placeholder}</span><input value={draft} onChange={(event) => setDraft(event.target.value)} placeholder={placeholder} type="search" /></label>;
}
