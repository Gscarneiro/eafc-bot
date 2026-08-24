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
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [position, setPosition] = useState("");
  const [tradeable, setTradeable] = useState<"all" | "tradeable" | "untradeable">("all");
  const { data, error, loading, refetch } = useData(
    () => fetchTime({ page, search, position, tradeable }),
    [page, search, position, tradeable],
  );
  const gate = asyncGate(loading, error, !!data, refetch);

  const [view, setView] = useState<"campo" | "tabela">(() => {
    try {
      return localStorage.getItem(VIEW_KEY) === "tabela" ? "tabela" : "campo";
    } catch {
      return "campo";
    }
  });
  const [lineup, setLineup] = useState<"atual" | "sugerida">("atual");
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
  const suggested = data.optimization?.moves?.length ? starters.map((s) => data.optimization.moves.find((m) => m.index === s.index)?.suggested ?? s) : starters;
  const displayedStarters = lineup === "sugerida" ? suggested : starters;
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
          <div className="toggle">
            <button className={view === "campo" ? "active" : ""} onClick={() => pitchOK && setView("campo")} disabled={!pitchOK} title={!pitchOK ? "A formação ainda não tem 11 slots reconhecidos para desenhar o campo" : undefined}>
              campo{!pitchOK ? " indisponível" : ""}
            </button>
            <button className={view === "tabela" ? "active" : ""} onClick={() => setView("tabela")}>
              tabela
            </button>
            {data.optimization?.status === "improved" && <button className={lineup === "sugerida" ? "active" : ""} onClick={() => setLineup(lineup === "sugerida" ? "atual" : "sugerida")}>melhor encaixe</button>}
          </div>
        }
      />

      <section>
        <h2>Titulares</h2>
        {showPitch ? (
          <Pitch formation={data.formation} starters={displayedStarters} />
        ) : (
          <RosterTable rows={displayedStarters.map((s) => ({ player: s.player, cardSlug: s.card_slug, position: s.position }))} />
        )}
        {data.optimization?.status === "improved" && <div className="notice"><strong>Melhor encaixe: +{data.optimization.gain.toFixed(1)} GG</strong><br />{data.optimization.chemistry_warning}</div>}
      </section>

      {(data.bench_total > 0 || search || position || tradeable !== "all") && (
        <section>
          <div className="section-title-row">
            <div><h2>Reservas</h2><p className="section-note">Cartas 88+ do clube, ordenadas por GG Rating.</p></div>
            <span className="count-label">{data.bench_total} encontradas</span>
          </div>
          <div className="roster-filters" aria-label="Filtrar reservas">
            <label><span>Buscar</span><input value={search} onChange={(e) => { setSearch(e.target.value); setPage(1); }} placeholder="Nome da carta" /></label>
            <label><span>Posição</span><select value={position} onChange={(e) => { setPosition(e.target.value); setPage(1); }}><option value="">Todas</option>{["GK", "RB", "CB", "LB", "RWB", "LWB", "CDM", "CM", "CAM", "RM", "LM", "RW", "LW", "CF", "ST"].map((p) => <option key={p}>{p}</option>)}</select></label>
            <label><span>Status</span><select value={tradeable} onChange={(e) => { setTradeable(e.target.value as typeof tradeable); setPage(1); }}><option value="all">Todas</option><option value="tradeable">Negociáveis</option><option value="untradeable">Inegociáveis</option></select></label>
          </div>
          <RosterTable rows={bench.map((b) => ({ player: b.player, cardSlug: b.card_slug }))} />
          <Pagination page={data.bench_page} pageSize={data.bench_page_size} total={data.bench_total} onPage={setPage} />
        </section>
      )}
    </div>
  );
}

function Pagination({ page, pageSize, total, onPage }: { page: number; pageSize: number; total: number; onPage: (next: number) => void }) {
  const pages = Math.max(1, Math.ceil(total / pageSize));
  if (pages <= 1) return null;
  return <nav className="pagination" aria-label="Paginação de reservas"><span>Página {page} de {pages}</span><button disabled={page <= 1} onClick={() => onPage(page - 1)}>Anterior</button><button disabled={page >= pages} onClick={() => onPage(page + 1)}>Próxima</button></nav>;
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
