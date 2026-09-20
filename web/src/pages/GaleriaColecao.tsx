import { useState } from "react";
import { useData } from "../useData";
import { fetchGaleriaColecao, saveGaleriaCardOverride } from "../api";
import { Link } from "react-router-dom";
import PageHeader from "../components/PageHeader";

export default function GaleriaColecao() {
  const { data, loading, error, refetch } = useData(fetchGaleriaColecao, []);
  const [saving, setSaving] = useState<number | null>(null);
  const [message, setMessage] = useState("");
  const setOwner = async (id: number, value: boolean) => {
    setSaving(id); setMessage("");
    try { await saveGaleriaCardOverride(id, { first_owner: value }); refetch(); setMessage("Correção salva e Gallery recalculada."); }
    catch (err) { setMessage(err instanceof Error ? err.message : "Não foi possível salvar a correção."); }
    finally { setSaving(null); }
  };
  return <div className="wrap galeria-page"><PageHeader eyebrow="fut gallery · coleção" title="Minha coleção" meta={data ? `${data["@odata.count"]} cartas conhecidas` : "histórico acumulado"} actions={<Link className="btn ghost" to="/galeria">voltar à Gallery</Link>} />
    {message && <p className="hint" role="status">{message}</p>}{loading && <p className="hint">Carregando coleção...</p>}{error && <div className="banner alert" role="alert">{error.message}</div>}
    <div className="panel"><div className="tablewrap"><table><thead><tr><th>Carta</th><th>Versão</th><th>Item Score</th><th>Estado e procedência</th></tr></thead><tbody>{(data?.value ?? []).map((card) => <tr key={`${card.id}-${card.player_id || "card"}`}><td>{card.name || card.id}</td><td>{card.version || "—"}</td><td className="num">{card.item_score ? card.item_score.toLocaleString("pt-BR") : "desconhecido"}</td><td>{card.historical ? "histórica" : "atual"}{card.loan === true ? " · empréstimo" : card.loan === undefined ? " · empréstimo desconhecido" : ""}{card.first_owner === undefined ? <span> · primeiro dono desconhecido <button className="btn ghost" disabled={saving === card.id} onClick={() => setOwner(card.id, true)}>confirmar</button> <button className="btn ghost" disabled={saving === card.id} onClick={() => setOwner(card.id, false)}>marcar comprado</button></span> : card.first_owner ? " · primeiro dono confirmado" : " · primeiro dono falso"}{card.source ? ` · fonte ${card.source}` : ""}</td></tr>)}</tbody></table></div>{(data?.value?.length ?? 0) === 0 && !loading && <p className="hint" style={{ padding: 12 }}>A coleção será preenchida após a próxima sincronização.</p>}</div></div>;
}
