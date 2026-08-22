import { fetchMercado } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import TrendChart from "../components/TrendChart";
import { formatCoins, formatSigned } from "../format";
import { useData } from "../useData";
import "../shared.css";
import "./Mercado.css";

// Mercado é "/mercado": os upgrades já ordenados por ganho por moeda gasta
// (ver analyze.FindUpgrades) — a resposta chega pronta, esta tela não
// reordena nada. Um card fora do orçamento continua visível, só esmaecido
// — é uma meta pra hoje, não um erro (ver analyze.Upgrade.Affordable).
export default function Mercado() {
  const { data, error, loading, refetch } = useData(fetchMercado, []);
  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;
  if (!data) return null;

  const upgrades = data.upgrades ?? [];
  const series = data.price_series ?? {};

  return (
    <div className="wrap">
      <PageHeader title="Oportunidades de mercado" meta={`${upgrades.length} sugestões · ordenadas por ganho por moeda gasta`} />

      {upgrades.length === 0 ? (
        <EmptyState
          message="Nenhuma oportunidade dentro do orçamento hoje."
          hint="Aumente market.extra_budget na configuração, ou espere o mercado se mexer."
        />
      ) : (
        <div className="card-list">
          {upgrades.map((u, i) => {
            const pts = (series[String(u.candidate.id)] ?? [])
              .slice()
              .sort((a, b) => a.observed_at.localeCompare(b.observed_at))
              .map((pt) => ({ label: pt.observed_at, value: pt.coins }));
            return (
              <div className={`tx-card${u.affordable ? "" : " unaffordable"}`} key={i}>
                <div className="tx-head">
                  <Chip tone="turf">{u.slot}</Chip>
                  {u.candidate.image_url && <img className="tx-thumb" src={u.candidate.image_url} alt="" loading="lazy" />}
                  <span className="swap">
                    <strong>{u.current.common_name || u.current.name}</strong>
                    <span className="arrow">&rarr;</span>
                    <strong>{u.candidate.common_name || u.candidate.name}</strong>
                  </span>
                  <Chip tone="gain">{formatSigned(u.gain)} nota</Chip>
                </div>
                <div className="tx-row">
                  <span>{u.unpriced ? "custo desconhecido" : `${formatCoins(u.net_cost)} líquido`}</span>
                  {u.profit > 0 && <Chip tone="gain">lucro {formatCoins(u.profit)}</Chip>}
                  {!u.affordable && <Chip tone="alert">fora do orçamento</Chip>}
                  <span className="eff">{u.efficiency.toFixed(2)} pts / 10k moedas</span>
                </div>
                {u.rationale && u.rationale.length > 0 && (
                  <ul className="rationale">
                    {u.rationale.map((r, j) => (
                      <li key={j}>{r}</li>
                    ))}
                  </ul>
                )}
                {pts.length > 1 && (
                  <div className="spark">
                    <TrendChart data={pts} compact height={36} />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
