const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "ui/app.js"), "utf8");
function sourceBlock(start, end) {
  const from = source.indexOf(start);
  const to = source.indexOf(end, from);
  assert.ok(from >= 0 && to > from, `missing UI boundary: ${start}`);
  return source.slice(from, to);
}

function harness(items) {
  const elements = new Map([
    ["repository-candidates", { dataset: {}, innerHTML: "", textContent: "" }],
    ["path", { value: "C:\\Workspace\\Root\\" }],
  ]);
  const context = vm.createContext({
    document: { getElementById: id => elements.get(id) },
    mutationHeaders: () => ({ "X-Control-Room-Token": "test" }),
    request: async () => items,
  });
  const uiSource = [
    sourceBlock("  const escapeHTML =", "  const findingTone ="),
    sourceBlock("  const candidates =", "  async function openFolderPicker"),
  ].join("\n");
  vm.runInContext(`${uiSource}\nthis.ui = { discoverRepositories };`, context);
  return { ui: context.ui, elements };
}

test("repository discovery checks only the selected root and discloses nested candidates", async () => {
  const h = harness([
    { name: "root", path: "c:/workspace/root" },
    { name: "nested", path: "C:\\Workspace\\Root\\nested" },
  ]);
  await h.ui.discoverRepositories();
  const html = h.elements.get("repository-candidates").innerHTML;
  assert.match(html, /다른 발견 저장소/);
  assert.match(html, /name="repositoryPath"[^>]*value="c:\/workspace\/root"[^>]*checked/);
  assert.match(html, /<details class="repository-discovery-other">/);
  assert.match(html, /nested/);
  assert.match(html, /value="C:\\Workspace\\Root\\nested"[^>]*>/);
  assert.doesNotMatch(html, /value="C:\\Workspace\\Root\\nested"[^>]*checked/);
});

test("repository discovery leaves every candidate unchecked when the selected folder is not a repository", async () => {
  const h = harness([{ name: "nested", path: "C:\\Workspace\\Root\\nested" }]);
  await h.ui.discoverRepositories();
  const html = h.elements.get("repository-candidates").innerHTML;
  assert.match(html, /다른 발견 저장소/);
  assert.doesNotMatch(html, /value="C:\\Workspace\\Root\\nested"[^>]*checked/);
});
