import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { fetchTime } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import PageHeader from "../components/PageHeader";
import Pitch, { canDrawPitch } from "../components/Pitch";
import { useData } from "../useData";
import type { ClubPlayer, Position } from "../types";
import "../shared.css";

const VIEW_KEY = "eafc-bot:time-view";

// Time é a tela "/time": o XI titular (posição do slot físico — pode
// divergir da posição natural da carta, ver domain.SquadSlot) e o banco.
// O titular padrão é o campo desenhado na formação de verdade; a tabela
// continua disponível pelo toggle — melhor pra varrer números em série.
export default function Time() {
  const { data, error, loading, refetch } = useData(fetchTime, []);
  const gate = asyncGate(loading, error, !!data, refetch);

  const [view, setView] = useState<"campo" | "tabela">(() => {
    try {
      return localStorage.getItem(VIEW_KEY) === "tabela" ? "tabela" : "campo";
    } catch {
      return "campo";
    }
  });
  useEffect(() => {
    try {
      localStorage.setItem(VIEW_KEY, view);
    } catch {
      // Sem storage disponível (navegação privada, política do navegador)
      // a tela ainda funciona — só esquece a preferência ao recarregar.
    }
  }, [view]);

  if (gate) return gate;
  if (!data) return null;

  const starters = data.starters ?? [];
  const bench = data.bench ?? [];
  const pitchOK = canDrawPitch(data.formation || "", starters.length);
  const showPitch = pitchOK && view === "campo";

  return (
    <div className="wrap">
      <PageHeader
        eyebrow={`formação ${data.formation || "—"}`}
        title="Meu time"
        meta={`${starters.length} titulares · ${bench.length} reservas`}
        actions={
          pitchOK ? (
            <div className="toggle">
              <button className={view === "campo" ? "active" : ""} onClick={() => setView("campo")}>
                escalação
              </button>
              <button className={view === "tabela" ? "active" : ""} onClick={() => setView("tabela")}>
                tabela
              </button>
            </div>
          ) : undefined
        }
      />

      <section>
        <h2>Titulares</h2>
        {showPitch ? (
          <Pitch formation={data.formation} starters={starters} />
        ) : (
          <RosterTable rows={starters.map((s) => ({ player: s.player, cardSlug: s.card_slug, position: s.position }))} />
        )}
      </section>

      {bench.length > 0 && (
        <section>
          <h2>Reservas</h2>
          <RosterTable rows={bench.map((b) => ({ player: b.player, cardSlug: b.card_slug }))} />
        </section>
      )}
    </div>
  );
}

interface Row {
  player: ClubPlayer;
  cardSlug?: string;
  position?: Position;
}

function RosterTable({ rows }: { rows: Row[] }) {
  return (
    <div className="tablewrap">
      <table>
        <thead>
          <tr>
            <th></th>
            <th>Posição</th>
            <th>Carta</th>
            <th className="num">Overall</th>
            <th className="num">GG Rating</th>
            <th>Química</th>
          </tr>
        </thead>
        <tbody>
          {rows.map(({ player: p, cardSlug, position }) => (
            <tr key={p.id}>
              <td>{p.image_url && <img className="thumb" src={p.image_url} alt="" loading="lazy" />}</td>
              <td>
                {position ?? p.position}
                {p.out_of_pos ? " *" : ""}
              </td>
              <td className="namecell">
                {cardSlug ? (
                  <Link to={`/time/${cardSlug}`}>{p.common_name || p.name}</Link>
                ) : (
                  <span title="Abaixo do overall mínimo analisado carta a carta">{p.common_name || p.name}</span>
                )}
                {p.untradeable && <Chip tone="flat"> untradeable</Chip>}
              </td>
              <td className="num">{p.rating}</td>
              <td className="num">{p.gg_rating ? p.gg_rating.toFixed(1) : "—"}</td>
              <td>{p.chemistry}/3</td>
            </tr>
          ))}
        </tbody>
      </table>
      {rows.length === 0 && <div className="empty">Nenhuma carta aqui.</div>}
    </div>
  );
}
