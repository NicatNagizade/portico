import assert from "node:assert/strict";
import test from "node:test";
import { parsePrimaryKeyConfig } from "../src/connectors/primaryKey.js";
import { filterRows } from "../src/connectors/shared/filters.js";
import {
  buildFilterBy,
  compileFilters,
} from "../src/connectors/typesense/search.js";
import { parsePage, totalPages } from "../src/utils/pagination.js";
import { open, redact, seal, setKey } from "../src/utils/secretbox.js";

test("secretbox matches its own seal", () => {
  setKey("test-key");
  const sealed = seal({ host: "localhost", password: "s3cret", api_key: "k" });
  assert.equal(sealed.host, "localhost");
  assert.match(String(sealed.password), /^enc:v1:/);
  assert.equal(open(sealed).password, "s3cret");
  assert.equal(redact(open(sealed)).password, "");
  assert.equal(redact(open(sealed)).api_key, "");
});

test("primary key shapes", () => {
  assert.equal(parsePrimaryKeyConfig({}).configured, false);
  assert.deepEqual(parsePrimaryKeyConfig({ primary_key: "uid" }).source, [
    "uid",
  ]);
  assert.equal(parsePrimaryKeyConfig({ primary_key: "uid" }).intField, "");
  assert.equal(
    parsePrimaryKeyConfig({ primary_key_int: "sort_id" }).intField,
    "sort_id",
  );
  assert.deepEqual(
    parsePrimaryKeyConfig({ primary_key: ["user_id", "post_id"] }).source,
    ["user_id", "post_id"],
  );
});

test("doc filters AND and dotted paths", () => {
  const rows = [
    { id: 1, user: { name: "ada" } },
    { id: 2, user: { name: "bea" } },
  ];
  const matched = filterRows(rows, [
    { column: "user.name", operator: "eq", value: "ada" },
  ]);
  assert.equal(matched.length, 1);
  assert.equal(matched[0].id, 1);
});

test("typesense filters match field types", () => {
  const fields: Array<{
    name: string;
    type: "int64" | "int64[]" | "string" | "bool";
  }> = [
    { name: "user_id", type: "int64" },
    { name: "title", type: "string" },
    { name: "user.bio", type: "string" },
    { name: "active", type: "bool" },
    { name: "comments.user_id", type: "int64[]" },
  ];
  assert.equal(
    buildFilterBy(
      [{ column: "user_id", operator: "eq", value: "1" }],
      "id_int",
      fields,
    ),
    "user_id:=1",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "user_id", operator: "neq", value: "1" }],
      "id_int",
      fields,
    ),
    "user_id:!=1",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "user_id", operator: "in", value: "1, 2" }],
      "id_int",
      fields,
    ),
    "user_id:[1, 2]",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "user_id", operator: "not_in", value: "1, 2" }],
      "id_int",
      fields,
    ),
    "user_id:!=[1, 2]",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "comments.user_id", operator: "in", value: "1, 2" }],
      "id_int",
      fields,
    ),
    "comments.user_id:[1, 2]",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "active", operator: "eq", value: "true" }],
      "id_int",
      fields,
    ),
    "active:=true",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "title", operator: "eq", value: "one" }],
      "id_int",
      fields,
    ),
    "title:=`one`",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "title", operator: "in", value: "one, two" }],
      "id_int",
      fields,
    ),
    "title:=[`one`, `two`]",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "title", operator: "is_null", value: "" }],
      "id_int",
      fields,
    ),
    "title:_missing",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "title", operator: "is_not_null", value: "" }],
      "id_int",
      fields,
    ),
    "title:!_missing",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "user_id", operator: "gt", value: "2" }],
      "id_int",
      fields,
    ),
    "user_id:>2",
  );
  assert.equal(
    buildFilterBy(
      [{ column: "title", operator: "gt", value: "Weekend" }],
      "id_int",
      fields,
    ),
    null,
  );
  assert.equal(
    compileFilters(
      [{ column: "title", operator: "like", value: "%Looking ahead #1%" }],
      "id_int",
      fields,
    ),
    null,
  );
  assert.equal(
    compileFilters(
      [{ column: "user.bio", operator: "like", value: "Bio_" }],
      "id_int",
      fields,
    ),
    null,
  );
});

test("pagination defaults", () => {
  assert.deepEqual(parsePage({}), { page: 1, pageSize: 20 });
  assert.equal(parsePage({ page_size: "500" }).pageSize, 100);
  assert.equal(totalPages(21, 20), 2);
});
