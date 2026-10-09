import { stringify } from "csv-stringify/sync";

export function toCSV(records: string[][]): string {
  return stringify(records);
}

export function csvCell(value: unknown): string {
  if (value == null) return "";
  if (typeof value === "string") return value;
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
