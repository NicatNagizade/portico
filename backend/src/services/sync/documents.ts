import { isActive, type Rule, type SyncJob } from "../../domain/types.js";
import { cloneRows } from "../../connectors/shared/rows.js";
import {
  applyPrimaryKeyOverride,
  parsePrimaryKeyConfig,
  type PrimaryKeyConfig,
} from "../../connectors/primaryKey.js";
import {
  ensureId,
  type Doc,
  type Filter,
  type SourceReader,
  type TableSchema,
} from "../../connectors/types.js";
import { applyFields } from "./fields.js";
import { enrichRelations } from "./relations/enrich.js";

export type SourceShape = { schema: TableSchema; pk: PrimaryKeyConfig };

/** Source table schema with the job's `primary_key` override applied. */
export async function loadSourceShape(
  source: SourceReader,
  job: SyncJob,
): Promise<SourceShape> {
  const pk = parsePrimaryKeyConfig(job.config ?? {});
  const schema = await source.schema(job.source_table);
  return { schema: applyPrimaryKeyOverride(schema, pk) ?? schema, pk };
}

export function activeFilters(rules: Rule[]): Filter[] {
  return rules
    .filter((rule) => isActive(rule.active))
    .map((rule) => ({
      column: rule.field,
      operator: rule.operator,
      value: rule.value,
    }));
}

/** Sets document ids and applies field overrides — the last step before writing. */
export function finishDocs(
  docs: Doc[],
  job: SyncJob,
  { schema, pk }: SourceShape,
  startIndex: number,
): void {
  ensureId(docs, schema, startIndex, pk.destination);
  applyFields(docs, job.fields);
}

/** Copies source rows, loads relations, and finishes them into destination docs. */
export async function buildDocs(
  source: SourceReader,
  job: SyncJob,
  shape: SourceShape,
  rows: Doc[],
  startIndex: number,
): Promise<Doc[]> {
  const docs = cloneRows(rows);
  await enrichRelations(source, job, shape.schema, docs);
  finishDocs(docs, job, shape, startIndex);
  return docs;
}
