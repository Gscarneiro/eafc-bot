export type Literal = string | number | boolean | null;

export type Filter =
  | { kind: "compare"; field: string; op: "eq" | "ne" | "gt" | "ge" | "lt" | "le" | "in"; value: Literal | Literal[] }
  | { kind: "function"; fn: "contains" | "startswith" | "endswith"; field: string; value: string }
  | { kind: "and" | "or"; left: Filter; right: Filter }
  | { kind: "not"; expression: Filter };

export interface OrderBy {
  field: string;
  desc?: boolean;
}

export interface ODataQuery {
  filter?: Filter;
  orderBy?: OrderBy[];
  search?: string;
  top?: number;
  skip?: number;
  count?: boolean;
}

function literal(value: Literal): string {
  if (value === null) return "null";
  if (typeof value === "boolean" || typeof value === "number") return String(value);
  return `'${value.replaceAll("'", "''")}'`;
}

function precedence(filter: Filter): number {
  if (filter.kind === "or") return 1;
  if (filter.kind === "and") return 2;
  if (filter.kind === "not") return 3;
  return 4;
}

export function formatFilter(filter: Filter, parent = 0): string {
  let result: string;
  switch (filter.kind) {
    case "compare":
      result = filter.op === "in"
        ? `${filter.field} in (${(Array.isArray(filter.value) ? filter.value : [filter.value]).map(literal).join(",")})`
        : `${filter.field} ${filter.op} ${literal(Array.isArray(filter.value) ? filter.value[0] ?? null : filter.value)}`;
      break;
    case "function":
      result = `${filter.fn}(${filter.field},${literal(filter.value)})`;
      break;
    case "not":
      result = `not ${formatFilter(filter.expression, precedence(filter))}`;
      break;
    case "and":
    case "or":
      result = `${formatFilter(filter.left, precedence(filter))} ${filter.kind} ${formatFilter(filter.right, precedence(filter))}`;
      break;
  }
  return precedence(filter) < parent ? `(${result})` : result;
}

type Token = { value: string; quoted: boolean };

function tokenize(input: string): Token[] {
  const tokens: Token[] = [];
  for (let i = 0; i < input.length;) {
    if (/\s/.test(input[i]!)) { i += 1; continue; }
    const char = input[i]!;
    if (char === "(") { tokens.push({ value: char, quoted: false }); i += 1; continue; }
    if (char === ")") { tokens.push({ value: char, quoted: false }); i += 1; continue; }
    if (char === ",") { tokens.push({ value: char, quoted: false }); i += 1; continue; }
    if (char === "'") {
      i += 1;
      let value = "";
      while (i < input.length) {
        if (input[i] === "'" && input[i + 1] === "'") { value += "'"; i += 2; continue; }
        if (input[i] === "'") { i += 1; break; }
        value += input[i++];
      }
      tokens.push({ value, quoted: true });
      continue;
    }
    const start = i;
    while (i < input.length && !/\s|[(),]/.test(input[i]!)) i += 1;
    tokens.push({ value: input.slice(start, i), quoted: false });
  }
  return tokens;
}

function parseLiteral(token: Token): Literal {
  if (token.quoted) return token.value;
  if (token.value === "null") return null;
  if (token.value === "true") return true;
  if (token.value === "false") return false;
  const numeric = Number(token.value);
  return Number.isNaN(numeric) ? token.value : numeric;
}

export function parseFilter(input: string): Filter {
  const tokens = tokenize(input);
  let index = 0;
  const peek = () => tokens[index];
  const next = () => tokens[index++];
  const expect = (value: string) => {
    const token = next();
    if (!token || token.value.toLowerCase() !== value) throw new Error(`filtro inválido: esperava ${value}`);
    return token;
  };
  const parseOr = (): Filter => {
    let result = parseAnd();
    while (peek()?.value.toLowerCase() === "or") { next(); result = { kind: "or", left: result, right: parseAnd() }; }
    return result;
  };
  const parseAnd = (): Filter => {
    let result = parseUnary();
    while (peek()?.value.toLowerCase() === "and") { next(); result = { kind: "and", left: result, right: parseUnary() }; }
    return result;
  };
  const parseUnary = (): Filter => {
    if (peek()?.value.toLowerCase() === "not") { next(); return { kind: "not", expression: parseUnary() }; }
    if (peek()?.value === "(") { next(); const result = parseOr(); expect(")"); return result; }
    const first = next();
    if (!first) throw new Error("filtro vazio");
    const functionName = first.value.toLowerCase();
    if (["contains", "startswith", "endswith"].includes(functionName) && peek()?.value === "(") {
      next(); const field = next(); expect(","); const value = next(); expect(")");
      if (!field || !value || !value.quoted) throw new Error("a função exige campo e texto literal");
      return { kind: "function", fn: functionName as "contains" | "startswith" | "endswith", field: field.value, value: value.value };
    }
    const op = next();
    if (!op) throw new Error("filtro inválido: operador ausente");
    if (op.value.toLowerCase() === "in") {
      expect("("); const values: Literal[] = [];
      do { const value = next(); if (!value) throw new Error("lista in incompleta"); values.push(parseLiteral(value)); } while (peek()?.value === "," && Boolean(next()));
      expect(")");
      return { kind: "compare", field: first.value, op: "in", value: values };
    }
    if (!["eq", "ne", "gt", "ge", "lt", "le"].includes(op.value.toLowerCase())) throw new Error(`operador não suportado: ${op.value}`);
    const value = next();
    if (!value) throw new Error("literal ausente");
    return { kind: "compare", field: first.value, op: op.value.toLowerCase() as "eq" | "ne" | "gt" | "ge" | "lt" | "le", value: parseLiteral(value) };
  };
  const result = parseOr();
  if (index < tokens.length) throw new Error(`token inesperado: ${tokens[index]!.value}`);
  return result;
}

export function toSearchParams(query: ODataQuery): URLSearchParams {
  const params = new URLSearchParams();
  if (query.filter) params.set("$filter", formatFilter(query.filter));
  if (query.orderBy?.length) params.set("$orderby", query.orderBy.map((order) => `${order.field}${order.desc ? " desc" : " asc"}`).join(","));
  if (query.search?.trim()) params.set("$search", query.search.trim());
  if (query.top !== undefined) params.set("$top", String(Math.max(0, Math.floor(query.top))));
  if (query.skip !== undefined) params.set("$skip", String(Math.max(0, Math.floor(query.skip))));
  if (query.count !== undefined) params.set("$count", String(query.count));
  return params;
}

export function fromSearchParams(params: URLSearchParams): ODataQuery {
  const result: ODataQuery = {};
  const rawFilter = params.get("$filter");
  if (rawFilter) result.filter = parseFilter(rawFilter);
  const rawOrder = params.get("$orderby");
  if (rawOrder) result.orderBy = rawOrder.split(",").map((item) => { const [field, direction] = item.trim().split(/\s+/); return { field: field!, desc: direction?.toLowerCase() === "desc" }; });
  result.search = params.get("$search") ?? undefined;
  const top = Number(params.get("$top"));
  const skip = Number(params.get("$skip"));
  if (Number.isFinite(top) && top > 0) result.top = top;
  if (Number.isFinite(skip) && skip >= 0) result.skip = skip;
  result.count = params.get("$count") === "true";
  return result;
}
