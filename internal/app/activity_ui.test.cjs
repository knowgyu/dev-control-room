const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "ui/app.js"), "utf8");
const from = source.indexOf("  const activityNoopKey =");
const to = source.indexOf("  function renderServiceRecovery", from);
assert.ok(from >= 0 && to > from, "missing activity UI boundary");
const activitySource = source.slice(from, to);

function harness(events) {
  const element = { innerHTML: "" };
  const context = vm.createContext({
    state: { events },
    document: { getElementById: id => id === "events" ? element : null },
    escapeHTML: value => String(value ?? "").replace(/[&<>\"']/g, character => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;" })[character]),
    formatDate: value => value,
    localize: value => value,
  });
  vm.runInContext(`${activitySource}\nthis.ui = { activityNoopKey, collapseActivityEvents, renderActivity };`, context);
  return { ui: context.ui, element };
}

const scan = (occurredAt, trigger, failedProjectCount = 0) => ({
  spec: {
    type: "diagnosis.scan.completed",
    summary: "scheduled scan completed for 0 project(s)",
    data: { trigger, project_count: 0, failed_project_count: failedProjectCount },
    occurredAt,
  },
});

test("activity collapses only consecutive equivalent empty scans and keeps source events", () => {
  const events = [
    scan("2026-09-13T01:00:00Z", "scheduled"),
    scan("2026-09-13T02:00:00Z", "manual"),
    { spec: { type: "repository.updated", summary: "Repository updated", occurredAt: "2026-09-13T03:00:00Z" } },
    scan("2026-09-13T04:00:00Z", "startup"),
  ];
  const h = harness(events);
  const rows = h.ui.collapseActivityEvents(events);
  assert.equal(rows.length, 3);
  assert.equal(rows[0].repeatCount, 1);
  assert.equal(rows[1].item.spec.type, "repository.updated");
  assert.equal(rows[2].repeatCount, 2);
  assert.equal(events.length, 4);
  h.ui.renderActivity();
  assert.match(h.element.innerHTML, /반복 2회/);
  assert.equal((h.element.innerHTML.match(/diagnosis\.scan\.completed/g) || []).length, 2);
});

test("activity does not merge empty scans with different failure evidence", () => {
  const events = [
    scan("2026-09-13T01:00:00Z", "scheduled", 1),
    scan("2026-09-13T02:00:00Z", "manual", 0),
  ];
  const h = harness(events);
  const rows = h.ui.collapseActivityEvents(events);
  assert.equal(rows.length, 2);
  assert.equal(rows.every(row => row.repeatCount === 1), true);
});
