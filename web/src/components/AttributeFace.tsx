import type { Attributes, Position } from "../types";
import "./AttributeFace.css";

interface AttributeFaceProps {
  attributes: Attributes;
  position: Position;
}

// As mesmas seis colunas, dois vocabulários — o goleiro reaproveita os
// campos de atributo de linha com outro significado (armadilha registrada
// no CLAUDE.md: pac=DIV, sho=HAN, pas=KIC, dri=REF, def=SPD, phy=POS).
// Rotular como PAC/SHO num goleiro erra os seis atributos de uma vez.
const OUTFIELD = [
  { key: "pace", label: "PAC" },
  { key: "shooting", label: "SHO" },
  { key: "passing", label: "PAS" },
  { key: "dribbling", label: "DRI" },
  { key: "defending", label: "DEF" },
  { key: "physical", label: "PHY" },
] as const;

const GOALKEEPER = [
  { key: "pace", label: "DIV" },
  { key: "shooting", label: "HAN" },
  { key: "passing", label: "KIC" },
  { key: "dribbling", label: "REF" },
  { key: "defending", label: "SPD" },
  { key: "physical", label: "POS" },
] as const;

export default function AttributeFace({ attributes, position }: AttributeFaceProps) {
  const cols = position === "GK" ? GOALKEEPER : OUTFIELD;
  return (
    <div className="attr-face">
      {cols.map((c) => {
        const value = attributes[c.key];
        return (
          <div className="attr-row" key={c.key}>
            <span className="attr-label">{c.label}</span>
            <span className="attr-bar">
              <span className="attr-fill" style={{ width: `${Math.min(100, (value / 99) * 100)}%` }} />
            </span>
            <span className="attr-value">{value}</span>
          </div>
        );
      })}
    </div>
  );
}
