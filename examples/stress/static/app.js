"use strict";

const byId = (id) => document.getElementById(id);
const number = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 });
const decimal = new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 });
const formatMS = (value) => {
  if (value > 0 && value < 0.001) return `${decimal.format(value * 1000000)} ns`;
  if (value > 0 && value < 1) return `${decimal.format(value * 1000)} µs`;
  return `${decimal.format(value)} ms`;
};
const formatMiB = (value) => `${decimal.format(value / 1048576)} MiB`;
const formatBytes = (value) => value >= 1048576 ? formatMiB(value) : `${decimal.format(value / 1024)} KiB`;
const titleCase = (value) => value.charAt(0).toUpperCase() + value.slice(1);
let latest = null;
let lastLogSequence = 0;
let submitting = false;
let currentRun = 0;
let previousResultSignature = "";

async function requestJSON(path, options = {}) {
  const response = await fetch(path, options);
  const text = await response.text();
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    throw new Error(text.trim() || `Request failed (${response.status})`);
  }
  if (!response.ok) {
    throw new Error(data.error || `Request failed (${response.status})`);
  }
  return data;
}

function updateDashboard(state) {
  latest = state;
  const metrics = state.metrics;
  const busy = state.state === "preparing" || state.state === "running";
  byId("run-button").disabled = busy || submitting;
  byId("stop-button").disabled = !busy;
  byId("run-button").lastChild.textContent = busy ? " Test in progress" : "Run Test";
  const labels = {
    idle: "Ready to test",
    preparing: `Preparing: ${suiteName(state.suite)}`,
    running: `Running: ${suiteName(state.suite)}`,
    completed: "Run complete · verified",
    cancelled: "Run cancelled",
    failed: "Run failed · inspect output",
  };
  byId("run-status").textContent = labels[state.state] || state.state;
  byId("run-indicator").className = `run-indicator ${state.state}`;
  byId("metric-components").textContent = state.components ? number.format(state.components) : "—";
  byId("metric-nodes").textContent = state.nodes ? number.format(state.nodes) : "—";
  byId("metric-throughput").textContent = metrics.total_operations ? number.format(metrics.ops_per_second) : "—";
  byId("throughput-unit").textContent = state.unit || "Awaiting a workload";
  byId("throughput-label").textContent = "Throughput / s";
  byId("metric-render").textContent = metrics.renders ? `≤ ${formatMS(metrics.render_p95_ms)}` : "—";
  byId("metric-heap").textContent = formatMiB(state.current_heap_bytes);
  byId("metric-failures").textContent = state.run_id ? number.format(metrics.failed_operations) : "—";
  byId("metric-failures").className = metrics.failed_operations ? "red" : "green";
  byId("runtime-details").textContent = `${state.goroutines} goroutines · ${metrics.gc_cycles} GC cycles`;
  byId("progress-label").textContent = state.planned
    ? `${number.format(metrics.successful_operations)} / ${number.format(state.planned)} verified · ${state.workers} workers`
    : "No test running";

  if (state.run_id !== currentRun) {
    currentRun = state.run_id;
    byId("live-output").replaceChildren();
    lastLogSequence = 0;
  }
  for (const [id, status] of Object.entries(state.statuses)) {
    const row = document.querySelector(`[data-suite="${id}"]`);
    row.dataset.status = status;
    const badge = document.createElement("span");
    badge.textContent = titleCase(status);
    row.querySelector(".suite-status").replaceChildren(badge);
  }
  appendLogs(state.logs || []);
  updateResults(state.results || []);
  updateDetails(metrics);
  drawChart();
}

function suiteName(id) {
  const row = document.querySelector(`[data-suite="${id}"]`);
  return row ? row.firstElementChild.textContent : "workload";
}

function appendLogs(logs) {
  const output = byId("live-output");
  const follow = output.scrollHeight - output.scrollTop - output.clientHeight < 50;
  for (const entry of logs) {
    if (entry.sequence <= lastLogSequence) continue;
    const line = document.createElement("div");
    line.className = entry.level;
    const time = document.createElement("time");
    time.textContent = `[${new Date(entry.time).toLocaleTimeString(undefined, { hour12: false })}]`;
    line.append(time, document.createTextNode(entry.message));
    output.append(line);
    lastLogSequence = entry.sequence;
  }
  while (output.children.length > 160) output.firstElementChild.remove();
  if (follow) output.scrollTop = output.scrollHeight;
  byId("log-count").textContent = `${output.children.length} events`;
}

function updateResults(results) {
  const signature = results.map((result) => `${result.id}:${result.suite}:${result.status}`).join("|");
  byId("results-count").textContent = number.format(results.length);
  byId("download-results").disabled = results.length === 0;
  if (signature === previousResultSignature) return;
  previousResultSignature = signature;
  if (!results.length) return;

  const rows = results.slice().reverse().map((result) => {
    const row = document.createElement("tr");
    const name = document.createElement("td");
    name.textContent = result.name;
    const unit = document.createElement("small");
    unit.textContent = `${result.unit} · ${number.format(result.config.components)} instances`;
    name.append(unit);
    row.append(name);
    for (const value of [number.format(result.metrics.ops_per_second), `≤ ${formatMS(result.metrics.p95_ms)}`, formatMiB(result.metrics.heap_bytes)]) {
      const cell = document.createElement("td");
      cell.textContent = value;
      row.append(cell);
    }
    const status = document.createElement("td");
    status.className = result.status;
    status.textContent = result.status === "passed" ? "✓ Passed" : titleCase(result.status);
    status.title = result.error || "All correctness assertions passed";
    row.append(status);
    return row;
  });
  byId("results-body").replaceChildren(...rows);
}

function updateDetails(metrics) {
  if (!metrics.total_operations) return;
  const values = [
    `${number.format(metrics.successful_operations)} / ${number.format(metrics.total_operations)}`,
    [metrics.p50_ms, metrics.p95_ms, metrics.p99_ms].map(formatMS).join(" / "),
    `${number.format(metrics.dom_calls)} / ${number.format(metrics.renders)} / ${number.format(metrics.requests)}`,
    `${number.format(metrics.bytes_per_op)} B (process estimate)`,
    `${decimal.format(metrics.allocs_per_op)} (process estimate)`,
    `${metrics.gc_cycles} / ${formatMS(metrics.gc_pause_ms)}`,
    `${decimal.format(metrics.duration_seconds)} s / ${formatMS(metrics.average_ms)}`,
    `${formatBytes(metrics.output_bytes)} / ${metrics.after_gc_heap_bytes ? formatMiB(metrics.after_gc_heap_bytes) : "not sampled"}`,
  ];
  byId("detail-metrics").querySelectorAll("dd").forEach((cell, index) => { cell.textContent = values[index]; });
}

function svgNode(tag, attributes, text) {
  const element = document.createElementNS("http://www.w3.org/2000/svg", tag);
  for (const [key, value] of Object.entries(attributes)) element.setAttribute(key, value);
  if (text !== undefined) element.textContent = text;
  return element;
}

function drawChart() {
  const svg = byId("throughput-chart");
  const windowSeconds = Number(byId("chart-window").value);
  const all = latest?.chart || [];
  const end = all.length ? new Date(all[all.length - 1].time).getTime() : Date.now();
  const points = all.filter((point) => end - new Date(point.time).getTime() <= windowSeconds * 1000);
  const top = 15;
  const left = 66;
  const width = 675;
  const height = 169;
  const maxValue = Math.max(10, ...points.map((point) => point.value));
  const ceiling = Math.ceil(maxValue / 5) * 5;
  const nodes = [];
  const defs = svgNode("defs", {});
  const gradient = svgNode("linearGradient", { id: "chart-gradient", x1: "0", y1: "0", x2: "0", y2: "1" });
  gradient.append(svgNode("stop", { offset: "0%", "stop-color": "#3986ff", "stop-opacity": ".29" }), svgNode("stop", { offset: "100%", "stop-color": "#3986ff", "stop-opacity": ".03" }));
  defs.append(gradient);
  nodes.push(defs);
  for (let index = 0; index <= 4; index++) {
    const y = top + (height * index) / 4;
    nodes.push(svgNode("line", { x1: left, y1: y, x2: left + width, y2: y, stroke: "#e5eaf3", "stroke-width": "1" }));
    nodes.push(svgNode("text", { x: left - 12, y: y + 4, "text-anchor": "end", fill: "#7e8ba1", stroke: "none", "font-size": "12" }, number.format(ceiling * (1 - index / 4))));
  }
  for (let index = 0; index <= 6; index++) {
    const x = left + (width * index) / 6;
    nodes.push(svgNode("line", { x1: x, y1: top, x2: x, y2: top + height, stroke: "#edf0f6" }));
    nodes.push(svgNode("text", { x, y: top + height + 24, "text-anchor": "middle", fill: "#8995a8", stroke: "none", "font-size": "12" }, index === 6 ? "now" : `−${Math.round(windowSeconds * (1 - index / 6))}s`));
  }
  if (points.length) {
    const locations = points.map((point) => {
      const age = end - new Date(point.time).getTime();
      return [left + width * (1 - age / (windowSeconds * 1000)), top + height * (1 - point.value / ceiling)];
    });
    const line = locations.map(([x, y], index) => `${index ? "L" : "M"}${x.toFixed(2)},${y.toFixed(2)}`).join(" ");
    if (locations.length > 1) {
      nodes.push(svgNode("path", { d: `${line} L${locations[locations.length - 1][0]},${top + height} L${locations[0][0]},${top + height} Z`, fill: "url(#chart-gradient)", stroke: "none" }));
    }
    nodes.push(svgNode("path", { d: line, fill: "none", stroke: "#1b6aff", "stroke-width": "2.4" }));
    const last = locations[locations.length - 1];
    nodes.push(svgNode("circle", { cx: last[0], cy: last[1], r: "4", fill: "white", stroke: "#1b6aff", "stroke-width": "2" }));
  }
  svg.replaceChildren(...nodes);
  byId("chart-empty").hidden = points.length > 0;
  byId("chart-unit").textContent = latest?.unit ? `Verified ${latest.unit} / second` : "Verified operations / second";
  byId("chart-sample-count").textContent = points.length ? `${points.length} measured samples` : "No samples yet";
  byId("chart-title").textContent = latest?.suite ? `${suiteName(latest.suite)} Throughput` : "Workload Throughput";
}

byId("run-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (submitting) return;
  const form = new FormData(event.currentTarget);
  const config = Object.fromEntries(form.entries());
  for (const name of ["components", "iterations", "concurrency", "timeout_seconds"]) config[name] = Number(config[name]);
  byId("run-error").hidden = true;
  submitting = true;
  byId("run-button").disabled = true;
  try {
    updateDashboard(await requestJSON("/api/run", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(config) }));
  } catch (error) {
    byId("run-error").textContent = error.message;
    byId("run-error").hidden = false;
  } finally {
    submitting = false;
    if (latest) updateDashboard(latest);
    else byId("run-button").disabled = false;
  }
});

byId("stop-button").addEventListener("click", async () => {
  byId("stop-button").disabled = true;
  try {
    await requestJSON("/api/stop", { method: "POST" });
    byId("run-status").textContent = "Cancelling workload…";
  } catch (error) {
    byId("run-error").textContent = error.message;
    byId("run-error").hidden = false;
    byId("stop-button").disabled = false;
  }
});

byId("clear-output").addEventListener("click", () => {
  byId("live-output").replaceChildren();
  byId("log-count").textContent = "0 events";
});
byId("chart-window").addEventListener("change", drawChart);
byId("download-results").addEventListener("click", () => {
  const blob = new Blob([JSON.stringify(latest?.results || [], null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `live-components-lab-${new Date().toISOString().slice(0, 10)}.json`;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
});

document.querySelectorAll(".nav-item").forEach((link) => {
  link.addEventListener("click", () => {
    document.querySelectorAll(".nav-item").forEach((item) => item.classList.remove("active"));
    link.classList.add("active");
  });
});

const preview = { template: "banner", name: "Ada", children: 0, clones: 0, active: false, attribute: false, removed: false };
let previewRequest = null;
async function refreshPreview(message, action = "") {
  if (previewRequest) previewRequest.abort();
  previewRequest = new AbortController();
  try {
    preview.template = byId("preview-template").value;
    preview.name = byId("preview-name").value || "Ada";
    const data = await requestJSON(`/api/preview?${new URLSearchParams({ ...preview, action })}`, { signal: previewRequest.signal });
    // Only the local Go renderer supplies markup. User text uses SetTextContent there.
    byId("preview-stage").innerHTML = data.html;
    byId("preview-source").textContent = data.html;
    byId("preview-feedback").textContent = message;
  } catch (error) {
    if (error.name !== "AbortError") byId("preview-feedback").textContent = error.message;
  }
}

byId("preview-refresh").addEventListener("click", () => {
  Object.assign(preview, { children: 0, clones: 0, active: false, attribute: false, removed: false });
  refreshPreview("Fresh template parsed and rendered on the server.");
});
byId("preview-template").addEventListener("change", () => {
  Object.assign(preview, { children: 0, clones: 0, removed: false });
  refreshPreview("Template changed. This is real framework output.");
});
byId("preview-name").addEventListener("keydown", (event) => {
  if (event.key === "Enter") refreshPreview("SetTextContent updated the component.");
});
const previewMessages = {
  text: "SetTextContent updated the title and greeting.",
  attribute: "SetAttribute changed data-version. Inspect the rendered HTML.",
  class: "ClassList updated the underlying class attribute.",
  add: "AppendChild inserted a new child on the server.",
  remove: "Remove detached a child or the original button.",
  clone: "CloneNode(true) created an independent component copy.",
};
document.querySelectorAll("[data-preview]").forEach((button) => {
  button.addEventListener("click", () => {
    const action = button.dataset.preview;
    let serverAction = action;
    if (action === "attribute") preview.attribute = !preview.attribute;
    if (action === "class") preview.active = !preview.active;
    if (action === "add") preview.children = Math.min(8, preview.children + 1);
    if (action === "clone") preview.clones = Math.min(3, preview.clones + 1);
    if (action === "remove") {
      if (preview.clones) {
        preview.clones--;
        serverAction = "remove-clone";
      } else if (preview.children) {
        preview.children--;
        serverAction = "remove-child";
      } else {
        preview.removed = true;
      }
    }
    refreshPreview(previewMessages[action], serverAction);
  });
});
byId("preview-stage").addEventListener("click", (event) => {
  if (event.target.closest("button")) {
    preview.children = Math.min(8, preview.children + 1);
    refreshPreview("The component button requested a server-side child insertion.");
  }
});
byId("preview-source").textContent = byId("preview-stage").innerHTML;

const events = new EventSource("/api/events");
events.addEventListener("open", () => {
  byId("connection-dot").classList.add("connected");
  byId("connection-label").textContent = "Connected to Go runtime";
});
events.addEventListener("snapshot", (event) => updateDashboard(JSON.parse(event.data)));
events.addEventListener("error", () => {
  byId("connection-dot").classList.remove("connected");
  byId("connection-label").textContent = "Reconnecting to runtime";
});
requestJSON("/api/state").then(updateDashboard).catch(() => {
  byId("connection-label").textContent = "Runtime unavailable · retrying";
});
drawChart();
