import { isActive, type Relation } from "../../../domain/types.js";
import type { Doc } from "../../../connectors/types.js";

export function activeRelations(relations: Relation[]): Relation[] {
  return relations.filter((relation) => isActive(relation.active));
}

export function rootRelations(relations: Relation[]): Relation[] {
  return activeRelations(relations).filter((r) => r.parent_id == null);
}

export function childrenByParent(relations: Relation[]) {
  const children = new Map<number, Relation[]>();
  for (const relation of relations) {
    if (relation.parent_id == null) continue;
    const list = children.get(relation.parent_id);
    if (list) list.push(relation);
    else children.set(relation.parent_id, [relation]);
  }
  return children;
}

/** Active relations ordered parents-first, keeping input order within a depth. */
export function orderRelations(relations: Relation[]): Relation[] {
  const active = activeRelations(relations);
  const byId = new Map(active.map((relation) => [relation.id, relation]));
  const depths = new Map<number, number>();

  const depth = (id: number, ancestors: Set<number>): number => {
    const cached = depths.get(id);
    if (cached != null) return cached;
    if (ancestors.has(id)) throw new Error(`cyclic parent_id involving ${id}`);
    const relation = byId.get(id);
    if (!relation) throw new Error(`parent_id ${id} not found`);
    const value =
      relation.parent_id == null
        ? 0
        : depth(relation.parent_id, new Set(ancestors).add(id)) + 1;
    depths.set(id, value);
    return value;
  };

  return active
    .map((relation, index) => ({
      relation,
      index,
      depth: depth(relation.id, new Set()),
    }))
    .sort((a, b) => a.depth - b.depth || a.index - b.index)
    .map(({ relation }) => relation);
}

/** Every nested doc stored under key `name`, at any depth. */
export function collectRelationDocs(docs: Doc[], name: string): Doc[] {
  const matches: Doc[] = [];
  const pending = [...docs];
  const visited = new Set<Doc>();

  while (pending.length > 0) {
    const doc = pending.pop()!;
    if (visited.has(doc)) continue;
    visited.add(doc);

    for (const [key, value] of Object.entries(doc)) {
      const children = Array.isArray(value)
        ? value.filter(isDoc)
        : isDoc(value)
          ? [value]
          : [];
      if (key === name) matches.push(...children);
      pending.push(...children);
    }
  }
  return matches;
}

function isDoc(value: unknown): value is Doc {
  return value != null && typeof value === "object" && !Array.isArray(value);
}
