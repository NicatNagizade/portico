import type { Doc } from "../types.js";
import { parseInt64 } from "../../utils/values.js";

/** `id` is stored as `_id`, plus a numeric copy in `sortField` when it parses as an integer. */
export function toMongoDoc(doc: Doc, sortField: string): Doc {
  const out: Doc = {};
  for (const [key, value] of Object.entries(doc)) {
    if (key === "id" || (sortField && key === sortField)) continue;
    out[key] = value;
  }
  if (doc.id != null && String(doc.id) !== "") {
    out._id = String(doc.id);
    const n = parseInt64(doc.id);
    if (sortField && n != null) out[sortField] = n;
  }
  return out;
}

export function fromMongoDoc(doc: Doc, sortField: string): Doc {
  const out: Doc = {};
  for (const [key, value] of Object.entries(doc)) {
    if (sortField && key === sortField) continue;
    if (key === "_id") out.id = value == null ? value : String(value);
    else out[key] = value;
  }
  return out;
}
