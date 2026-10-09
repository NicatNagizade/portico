export type PageParams = { page: number; pageSize: number };

export function parsePage(query: Record<string, unknown>): PageParams {
  let page = 1;
  let pageSize = 20;
  const rawPage = query.page;
  const rawSize = query.page_size;
  if (typeof rawPage === "string" && rawPage !== "") {
    const n = Number(rawPage);
    if (Number.isInteger(n) && n > 0) page = n;
  }
  if (typeof rawSize === "string" && rawSize !== "") {
    const n = Number(rawSize);
    if (Number.isInteger(n) && n > 0) pageSize = n;
  }
  if (pageSize > 100) pageSize = 100;
  return { page, pageSize };
}

export function totalPages(total: number, pageSize: number): number {
  if (pageSize <= 0 || total <= 0) return 0;
  return Math.ceil(total / pageSize);
}

export function pageOf<T>(items: T[], total: number, p: PageParams) {
  return {
    items: items ?? [],
    page: p.page,
    page_size: p.pageSize,
    total,
    total_pages: totalPages(total, p.pageSize),
  };
}
