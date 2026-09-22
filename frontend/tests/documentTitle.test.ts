import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync("src/components/DocumentTitle.tsx", "utf8");
const mainSource = readFileSync("src/main.tsx", "utf8");

test("admin and public pages use the expected browser titles", () => {
  assert.match(source, /const ADMIN_TITLE = "Komari Lite Monitor"/);
  assert.match(source, /const PUBLIC_TITLE = "Komari Lite"/);
  assert.match(source, /publicInfo\?\.sitename\?\.trim\(\) \|\| PUBLIC_TITLE/);
  assert.match(source, /pathname\.startsWith\("\/admin\/"\)/);
});

test("document title handling is mounted for every application route", () => {
  assert.equal(mainSource.match(/<DocumentTitle \/>/g)?.length, 2);
});
