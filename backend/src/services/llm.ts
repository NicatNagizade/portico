import { ErrNotConfigured, invalid } from "../domain/errors.js";
import { ruleNeedsValue, validRuleOperator } from "../domain/types.js";
import { errorMessage } from "../utils/values.js";

export type Completer = {
  complete(system: string, user: string): Promise<string>;
};

const SYSTEM = `You help fill explore UI filters and visible fields from a short user request.

Return ONLY a JSON object with this shape:
{"filters":[{"field":"...","operator":"...","value":"..."}],"fields":["col1","col2"] or null}

Rules:
- filters are AND'd. Use [] when the user does not ask for filters.
- operators must be one of: eq, neq, gt, gte, lt, lte, in, not_in, like, is_null, is_not_null
- for is_null / is_not_null, set value to ""
- for in / not_in, value is a comma-separated list
- for like, include % wildcards when useful (e.g. "%john%")
- field names MUST be chosen from the provided filter_fields list (exact spelling)
- fields: null means show all columns; otherwise pick a subset from the provided fields list (exact spelling)
- if the user only mentions filters, set fields to null
- if the user only mentions columns, set filters to []
- do not invent field names that are not in the lists`;

export function newClient(
  apiKey: string,
  baseURL: string,
  model: string,
): Completer | null {
  apiKey = apiKey.trim();
  if (!apiKey) return null;
  baseURL = (baseURL.trim() || "https://api.openai.com/v1").replace(/\/$/, "");
  model = model.trim() || "gpt-4o-mini";
  return {
    async complete(system, user) {
      const res = await fetch(baseURL + "/chat/completions", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          authorization: `Bearer ${apiKey}`,
        },
        body: JSON.stringify({
          model,
          temperature: 0,
          response_format: { type: "json_object" },
          messages: [
            { role: "system", content: system },
            { role: "user", content: user },
          ],
        }),
        signal: AbortSignal.timeout(60_000),
      });
      const raw = await res.text();
      const parsed = JSON.parse(raw) as {
        choices?: Array<{ message?: { content?: string } }>;
        error?: { message?: string };
      };
      if (parsed.error?.message)
        throw new Error(`openai: ${parsed.error.message}`);
      if (!res.ok) throw new Error(`openai http ${res.status}: ${raw.trim()}`);
      const content = parsed.choices?.[0]?.message?.content?.trim();
      if (!content) throw new Error("empty openai response");
      return content;
    },
  };
}

export async function suggestExplore(
  client: Completer | null,
  input: { prompt?: string; filter_fields?: string[]; fields?: string[] },
): Promise<{ filters: Filter[]; fields: string[] | null }> {
  if (!client) throw ErrNotConfigured;
  const prompt = (input.prompt ?? "").trim();
  if (!prompt) throw invalid("prompt is required");
  const filterOpts = unique(input.filter_fields ?? []);
  const fieldOpts = unique(input.fields ?? []);
  const raw = await client.complete(
    SYSTEM,
    `User request:\n${prompt}\n\nAvailable filter_fields:\n${filterOpts.join("\n") || "(none)"}\n\nAvailable fields (columns):\n${fieldOpts.join("\n") || "(none)"}`,
  );
  return parseSuggest(raw, filterOpts, fieldOpts);
}

function parseSuggest(raw: string, filterOpts: string[], fieldOpts: string[]) {
  raw = stripFence(raw);
  let parsed: { filters?: Filter[]; fields?: string[] | null };
  try {
    parsed = JSON.parse(raw);
  } catch (err) {
    throw invalid(`invalid ai json: ${errorMessage(err)}`);
  }
  const filterIndex = new Map(filterOpts.map((n) => [n.toLowerCase(), n]));
  const fieldIndex = new Map(fieldOpts.map((n) => [n.toLowerCase(), n]));
  const filters: Filter[] = [];
  for (const f of parsed.filters ?? []) {
    const field = filterIndex.get((f.field ?? "").trim().toLowerCase());
    const op = (f.operator ?? "").trim();
    if (!field || !validRuleOperator(op)) continue;
    let value = (f.value ?? "").trim();
    if (!ruleNeedsValue(op)) value = "";
    else if (!value) continue;
    filters.push({ field, operator: op, value });
  }
  if (parsed.fields == null) return { filters, fields: null };
  const seen = new Set<string>();
  const picked: string[] = [];
  for (const name of parsed.fields) {
    const canon = fieldIndex.get(String(name).trim().toLowerCase());
    if (!canon || seen.has(canon)) continue;
    seen.add(canon);
    picked.push(canon);
  }
  return {
    filters,
    fields:
      picked.length === fieldOpts.length && fieldOpts.length > 0
        ? null
        : picked,
  };
}

function stripFence(s: string): string {
  s = s.trim();
  if (!s.startsWith("```")) return s;
  s = s.slice(3).trim();
  if (s.toLowerCase().startsWith("json")) s = s.slice(4).trim();
  const end = s.lastIndexOf("```");
  if (end >= 0) s = s.slice(0, end);
  return s.trim();
}

function unique(values: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const value of values) {
    const text = value.trim();
    const key = text.toLowerCase();
    if (!text || seen.has(key)) continue;
    seen.add(key);
    out.push(text);
  }
  return out;
}

type Filter = { field: string; operator: string; value: string };
