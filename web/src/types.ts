// Espelha, campo a campo, as tags `json:"..."` do lado Go — as structs de
// domínio (internal/domain/*.go) e os envelopes por rota (internal/api/api.go).
// Mudou uma tag lá, muda aqui — as duas pontas não têm geração automática
// entre si.
//
// Convenção que se repete o arquivo inteiro: um slice Go sem `omitempty`
// serializa `null` quando vazio, não `[]`; um campo COM `omitempty` (`?`
// aqui) pode estar AUSENTE. Sempre tratar listas com `?? []` antes de
// `.length`/`.map` (ver src/format.ts).

export type Position =
  | "GK"
  | "RB"
  | "LB"
  | "CB"
  | "RWB"
  | "LWB"
  | "CDM"
  | "CM"
  | "CAM"
  | "RM"
  | "LM"
  | "RW"
  | "LW"
  | "CF"
  | "ST";

export interface Attributes {
  pace: number;
  shooting: number;
  passing: number;
  dribbling: number;
  defending: number;
  physical: number;
}

export interface PlayStyle {
  name: string;
  plus: boolean;
}

export interface Price {
  coins: number;
  extinct: boolean;
  is_sbc: boolean;
  updated_at: string;
}

export interface Player {
  id: number;
  slug: string;
  name: string;
  common_name: string;
  rating: number;
  position: Position;
  alt_positions: Position[] | null;
  version: string;
  club: string;
  league: string;
  nation: string;
  attributes: Attributes;
  play_styles: PlayStyle[] | null;
  weak_foot: number;
  skill_moves: number;
  foot: string;
  height_cm: number;
  price: Price;
  cycle: string;
  released_at: string;
  gg_rating?: number;
  gg_rating_pos?: Position;
  image_url?: string;
  base_player_ea_id?: number;
  base_player_slug?: string;
  roles_plus?: number[];
  roles_plus_plus?: number[];
  // momentum_pct só vem preenchido em cartas lidas da rota de momentum —
  // quanto a carta caiu da própria média recente (o fut.gg já calcula;
  // ver analyze.FindInvestments). Ausente nas demais fontes.
  momentum_pct?: number;
}

export interface ClubPlayer extends Player {
  untradeable: boolean;
  in_squad: boolean;
  squad_slot: Position | "";
  out_of_pos: boolean;
  chemistry: number;
  evos_applied: string[] | null;
  evo_exhausted: boolean;
  contracts: number;
  acquired_at: string;
}

export interface EvolutionPath {
  steps: Player[]; // sempre "carta hoje" + "carta final", no mínimo
  chain: string[] | null;
  coin_cost: number;
  point_cost: number;
  is_expired: boolean;
  training_time: string;
}

export interface EvoPotential {
  path: EvolutionPath;
  final_overall: number;
  final_gg_rating: number;
  gg_rating_gain: number;
  gained_play_styles: PlayStyle[] | null;
  coin_cost: number;
  point_cost: number;
  training_time: string;
}

export interface PositionRoles {
  position: Position;
  plus_plus: string[] | null;
  plus: string[] | null;
}

export interface CardReport {
  slug: string;
  player: ClubPlayer;
  by_position: PositionRoles[] | null;
  // null quando a carta já está no teto das evoluções ativas — resposta
  // válida, não ausência de dado.
  best: EvoPotential | null;
  alternates: EvoPotential[] | null;
}

export interface PricePoint {
  ea_id: number;
  coins: number;
  extinct: boolean;
  observed_at: string;
}

export interface CardDetailResponse extends CardReport {
  price_series: PricePoint[] | null;
}

export interface Reward {
  kind: string;
  description: string;
  player_id: number;
  pack_value: number;
  coins: number;
  untradeable: boolean;
}

export interface Objective {
  id: string;
  name: string;
  group: string;
  tasks: string[] | null;
  rewards: Reward[] | null;
  expires_at: string;
  est_minutes: number;
  cycle: string;
  url: string;
}

// SBCChallenge é um desafio dentro de um SBC — o fut.gg já resolve o
// fodder mais barato que bate o requisito; requirements_text é texto
// livre, sem id de nação/liga/raridade (ver analyze.ParsedSBCRequirement
// no lado Go, que tenta reconhecer os padrões conhecidos).
export interface SBCChallenge {
  name: string;
  requirements_text: string[] | null;
  cheapest_solution_coins: number;
}

export interface SBC {
  id: string;
  name: string;
  group: string;
  repeatable: boolean;
  solution_cost: number;
  rewards: Reward[] | null;
  expires_at: string;
  cycle: string;
  url: string;
  challenges?: SBCChallenge[];
}

export interface NewsItem {
  id: string;
  title: string;
  summary: string;
  url: string;
  published_at: string;
  tags: string[] | null;
  player_ids: number[] | null;
}

export interface ClubDiff {
  added: ClubPlayer[] | null;
  removed: ClubPlayer[] | null;
  coins_delta: number;
}

export interface SnapshotSummary {
  date: string; // "2006-01-02"
  squad_score: number;
  coins: number;
}

export interface TopMove {
  kind: "upgrade" | "evolution";
  slot: Position;
  headline: string;
  gain: number;
  net_cost: number;
  link: string;
}

export interface StatusResponse {
  generated_at: string;
  cycle: string;
  squad_score: number;
  coins: number;
  raisable: number;
  weakest_slot: Position | "";
  weakest_name: string;
  weakest_gg_rating: number;
  top_move?: TopMove;
  diff: ClubDiff;
  new_cards: Player[] | null;
  news: NewsItem[] | null;
  sbcs: SBC[] | null;
  objectives: Objective[] | null;
  errors: string[] | null;
  history: SnapshotSummary[] | null;
}

export interface RosterCard {
  player: ClubPlayer;
  // Ausente (não vazio — `omitempty` do lado Go) quando a carta está abaixo
  // do cards_min_rating configurado: não tem análise de evolução, então não
  // tem página de detalhe pra linkar.
  card_slug?: string;
}

export interface StarterCard extends RosterCard {
  index: number;
  position_gg_rating?: number;
  position: Position; // slot físico — pode divergir da posição natural da carta
}

export interface TimeResponse {
  formation: string;
  starters: StarterCard[] | null;
  bench: RosterCard[] | null;
  bench_page: number;
  bench_page_size: number;
  bench_total: number;
  optimization: SquadOptimization;
}

export interface SquadMoveView { index:number; position:Position; current:StarterCard; suggested:StarterCard; current_gg_rating:number; suggested_gg_rating:number; gain:number }
export interface SquadAlternativeView { index:number; position:Position; players:StarterCard[] }
export interface SquadOptimization { status:string; reason?:string; current_average:number; suggested_average:number; gain:number; moves:SquadMoveView[]; alternatives:SquadAlternativeView[]; chemistry_warning:string }

export interface Upgrade {
  slot: Position;
  current: ClubPlayer;
  candidate: Player;
  current_score: number;
  candidate_score: number;
  gain: number;
  gross_cost: number;
  recoup: number;
  net_cost: number;
  profit: number;
  efficiency: number;
  affordable: boolean;
  unpriced: boolean;
  rationale: string[] | null;
}

// UpgradeFunnel explica uma lista de upgrades vazia carta a carta — sem
// isso, "0 sugestões" só sabe dizer que não achou nada, e quem lê fica
// adivinhando qual botão mexer (foi o que a mensagem antiga fazia ao culpar
// market.extra_budget, que nem chega a filtrar — ver analyze.UpgradeFunnel
// no lado Go). considered == 0 é o sinal de snapshot gravado antes deste
// campo existir; a tela cai no texto genérico nesse caso.
export interface UpgradeFunnel {
  considered: number;
  owned: number;
  sbc_only: number;
  unpriced: number;
  out_of_position: number;
  below_min_gain: number;
  suggested: number;
  min_gain: number;
  best_gain: number;
  best_slot: Position;
  best_name: string;
  has_best: boolean;
}

export interface MercadoResponse {
  upgrades: Upgrade[] | null;
  funnel: UpgradeFunnel;
  // chave é o eaId em texto — Go serializa map[int64]... como objeto JSON,
  // e chave de objeto JSON é sempre string.
  price_series: Record<string, PricePoint[]> | null;
}

export interface EvoRequirement {
  kind: string;
  int_value: number;
  strings: string[] | null;
  raw: string;
}

export interface EvoUpgradeStep {
  kind: string;
  attr: string;
  amount: number;
  max_value?: number;
  play_style: PlayStyle;
  position: Position | "";
  raw: string;
}

export interface EvoLevel {
  number: number;
  upgrades: EvoUpgradeStep[] | null;
  objectives: string[] | null;
}

export interface Evolution {
  id: string;
  slug: string;
  name: string;
  description: string;
  coin_cost: number;
  point_cost: number;
  requirements: EvoRequirement[] | null;
  levels: EvoLevel[] | null;
  expires_at: string;
  cycle: string;
  url: string;
}

export interface EvoMatch {
  evolution: Evolution;
  player: ClubPlayer;
  slot: Position;
  impact: number;
  current_gg_rating: number;
  final_gg_rating: number;
  result: Player;
  cost: number;
  affordable: boolean;
  acquisition: string;
  card_slug?: string;
  best_path: EvoPotential;
  alternates?: EvoPotential[];
  beats_starter: boolean;
  highlights: string[] | null;
}

export interface EvolucoesSummary {
  matches: number;
  players: number;
  starters: number;
  unaffordable: number;
  expiring_soon: number;
  by_acquisition: Record<string, number>;
}

export interface EvolucoesFilters {
  positions: Position[] | null;
  categories: string[] | null;
}

export interface EvolucoesResponse {
  matches: EvoMatch[] | null;
  total: number;
  page: number;
  page_size: number;
  pages: number;
  summary: EvolucoesSummary;
  filters: EvolucoesFilters;
}

export interface EvolutionFavoritesResponse {
  favorites: string[];
}

export interface JobStatus {
  running: boolean;
  last_started?: string;
  last_success?: string;
  last_error?: string;
}

export interface UISettings {
  market: {
    min_rating: number;
    max_rating: number;
    max_price: number;
    pages: number;
    per_page: number;
    extra_budget: number;
  };
  report: {
    min_gain: number;
    trend_window_hours: number;
    allow_out_of_position: boolean;
    allow_unpriced: boolean;
  };
  serve: {
    daily_at: string;
    stale_after_hours: number;
    retention_days: number;
    cards_min_rating: number;
    fast_refresh_minutes: number;
    momentum_window_hours: number;
    evolution_favorites: string;
  };
}

export interface ConfigResponse {
  settings: UISettings;
  env_locked: string[];
}

// --- Investimentos: agente de trading (ver analyze.FindInvestments /
// FindSellCandidates / FindFodderDemand no lado Go). Puramente
// consultivo — o bot nunca compra nem vende sozinho.

export interface Investment {
  candidate: Player;
  momentum_pct: number;
  implied_average: number;
  signal: "desconto" | "out-of-packs";
  rationale: string[] | null;
}

export interface InvestmentFunnel {
  considered: number;
  owned: number;
  not_tradeable: number;
  superseded_by_sibling: number;
  below_min_momentum: number;
  suggested: number;
  min_momentum_pct: number;
  best_rejected_pct: number;
  best_rejected_name: string;
  has_best_rejected: boolean;
}

export type SellRecommendation = "vender" | "segurar_potencial" | "promover" | "nao_vendavel";

export interface SellCandidate {
  player: ClubPlayer;
  recommendation: SellRecommendation;
  net_sell_value?: number;
  evo_gg_gain?: number;
  evo_cost?: number;
  rationale: string[] | null;
}

export interface SellFunnel {
  considered: number;
  not_tradeable: number;
  promotable: number;
  held_for_potential: number;
  suggested: number;
  min_evo_gg_gain: number;
}

export type FodderPhase = "recente" | "pico" | "esfriando" | "estavel" | "esvaziar" | "expirado";

export interface ParsedSBCRequirement {
  kind: "min_from" | "min_team_rating" | "min_rarity_count";
  value: string;
  min: number;
}

export interface FodderSignal {
  sbc_id: string;
  sbc_name: string;
  challenge: string;
  requirement: string;
  parsed?: ParsedSBCRequirement;
  pool_size: number; // -1 quando não computado, não "zero cartas"
  cost_coins: number;
  cost_change_pct: number;
  phase: FodderPhase;
  repeatable: boolean;
  expires_at: string;
  rationale: string[] | null;
}

export interface InvestimentosResponse {
  investments: Investment[] | null;
  investment_funnel: InvestmentFunnel;
  sell_candidates: SellCandidate[] | null;
  sell_funnel: SellFunnel;
  fodder_demand: FodderSignal[] | null;
}
