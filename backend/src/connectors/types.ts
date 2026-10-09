export type FieldType =
  "string" | "int64" | "float64" | "bool" | "object" | "object_array";

export type ColumnSchema = {
  name: string;
  type: FieldType;
  primaryKey?: boolean;
  columns?: ColumnSchema[];
};

export type TableSchema = { columns: ColumnSchema[] };

export type Filter = {
  column: string;
  operator: string;
  value: string;
  rel?: RelationSubquery;
};

export type RelationSubquery = {
  table: string;
  select: string;
  where: Filter[];
};

export type Order = { column: string; desc: boolean };

export type Doc = Record<string, unknown>;

export interface SourceReader {
  open(): Promise<void>;
  listTables(): Promise<string[]>;
  schema(table: string): Promise<TableSchema>;
  count(table: string, filters: Filter[]): Promise<number>;
  readChunks(
    table: string,
    chunkSize: number,
    filters: Filter[],
    fn: (docs: Doc[]) => Promise<void>,
  ): Promise<void>;
  query(
    table: string,
    columns: string[] | null,
    filters: Filter[],
    limit: number,
    offset: number,
    order: Order | null,
  ): Promise<Doc[]>;
  queryRows(
    table: string,
    columns: string[] | null,
    whereColumn: string,
    whereValues: unknown[],
  ): Promise<Doc[]>;
  supportsRelationFilters(): boolean;
  close(): Promise<void>;
}

export interface DestinationWriter {
  open(): Promise<void>;
  prepare(name: string, schema: TableSchema, config: unknown): Promise<void>;
  writeBatch(name: string, docs: Doc[]): Promise<void>;
  query?(
    name: string,
    filters: Filter[],
    limit: number,
    offset: number,
    order: Order | null,
  ): Promise<{ rows: Doc[]; total: number }>;
  applyJobConfig?(config: unknown): Promise<void>;
  close(): Promise<void>;
}

export function ensureId(
  docs: Doc[],
  schema: TableSchema | null,
  startIndex: number,
  destField: string,
): void {
  const field = destField.trim() === "" ? "id" : destField;
  const pkCols = (schema?.columns ?? [])
    .filter((c) => c.primaryKey)
    .map((c) => c.name);
  docs.forEach((doc, i) => {
    if (pkCols.length > 0) {
      doc[field] = pkCols.map((pk) => String(doc[pk])).join("_");
      if (field !== "id") doc.id = doc[field];
      return;
    }
    const current = doc[field];
    if (current != null && String(current) !== "") {
      doc[field] = String(current);
      if (field !== "id") doc.id = doc[field];
      return;
    }
    if (doc.id != null && String(doc.id) !== "") {
      doc.id = String(doc.id);
      if (field !== "id") doc[field] = doc.id;
      return;
    }
    const synthetic = String(startIndex + i);
    doc[field] = synthetic;
    if (field !== "id") doc.id = synthetic;
  });
}
