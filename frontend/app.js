/**
 * ============================================================================
 * Network PC Monitoring System - Frontend Application
 * ============================================================================
 * Client-side application for monitoring computer status on a network.
 * Provides UI for checking individual or all computers, and adding new ones.
 * ============================================================================
 */

"use strict";

// ============================================================================
// Configuration
// ============================================================================

const API_BASE_URL = "http://localhost:8081/api";

// ============================================================================
// State Management
// ============================================================================

let computers = [];
const fileTransferState = {
  sourceComputerId: "",
  targetComputerId: "",
  currentPath: "",
  parentPath: "",
  homePath: "",
  roots: [],
  entries: [],
  selectedPath: "",
};

// ============================================================================
// API Service Layer
// ============================================================================

const GENERIC_SERVER_RESPONSE_ERROR = "Invalid server response. Restart backend and try again.";

async function parseJSONOrThrow(response, fallbackMessage = GENERIC_SERVER_RESPONSE_ERROR) {
  const raw = await response.text();

  // Helpful diagnostics in the browser console so we can see what
  // the backend actually returned when parsing fails.
  const debugPrefix = "[API] Unexpected response payload";

  if (!raw || !raw.trim()) {
    console.error(`${debugPrefix}: empty body`, {
      status: response.status,
      statusText: response.statusText,
      url: response.url,
    });
    throw new Error("Server returned an empty response. Restart the backend and try again.");
  }

  let json;
  try {
    json = JSON.parse(raw);
  } catch (error) {
    const snippet = raw.slice(0, 200);
    console.error(`${debugPrefix}: non‑JSON content`, {
      status: response.status,
      statusText: response.statusText,
      url: response.url,
      bodyPreview: snippet,
    });

    // If the backend accidentally served HTML or a static file (e.g. old binary),
    // show a clearer message instead of the generic fallback.
    if (/\<html[\s>]/i.test(snippet)) {
      throw new Error(
        "Backend returned HTML instead of JSON. Make sure you are running the latest Go backend on port 8081."
      );
    }

    throw new Error(fallbackMessage);
  }

  // New: be tolerant of legacy backends that return bare data instead
  // of the wrapped { success, data, error } structure.
  if (!json || typeof json !== "object" || typeof json.success !== "boolean") {
    console.warn(
      `${debugPrefix}: JSON without expected { success, data, error } shape. Treating as success=true, data=json for backward compatibility.`,
      {
        status: response.status,
        statusText: response.statusText,
        url: response.url,
        parsed: json,
      }
    );

    return {
      success: true,
      data: json,
    };
  }

  return json;
}

/**
 * Fetches all computers from the backend API
 * @returns {Promise<Array>} Array of Computer objects
 * @throws {Error} If API call fails
 */
async function loadComputers() {
  try {
    const response = await fetch(`${API_BASE_URL}/computers`);
    const json = await parseJSONOrThrow(response);
    if (!json.success) throw new Error(json.error);
    return json.data;
  } catch (error) {
    console.error("Failed to load computers:", error);
    throw error;
  }
}

/**
 * Pings a single computer by ID
 * @param {string} id - Computer ID to ping
 * @returns {Promise<Object>} ComputerStatus object with status and checked time
 * @throws {Error} If API call fails
 */
async function pingOne(id) {
  try {
    const response = await fetch(`${API_BASE_URL}/ping/${id}`);
    const json = await parseJSONOrThrow(response);
    if (!json.success) throw new Error(json.error);
    return json.data;
  } catch (error) {
    console.error(`Failed to ping computer ${id}:`, error);
    throw error;
  }
}

/**
 * Pings all computers simultaneously
 * @returns {Promise<Array>} Array of ComputerStatus objects
 * @throws {Error} If API call fails
 */
async function pingAll() {
  try {
    const response = await fetch(`${API_BASE_URL}/ping-all`);
    const json = await parseJSONOrThrow(response);
    if (!json.success) throw new Error(json.error);
    return json.data;
  } catch (error) {
    console.error("Failed to ping all computers:", error);
    throw error;
  }
}

/**
 * Runs one command via /api/terminal/execute and returns command output.
 * @param {string} computerId - Computer ID
 * @param {string} command - Linux command
 * @returns {Promise<string>} command output text
 */
async function runTerminalCommand(computerId, command) {
  const response = await fetch(`${API_BASE_URL}/terminal/execute`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ computerId, command }),
  });

  const json = await parseJSONOrThrow(response);

  if (!json.success) {
    throw new Error(json.error || "Terminal command failed.");
  }

  return (json.data && json.data.output) || "";
}

async function listTransferFiles(computerId, path = "") {
  const params = new URLSearchParams();
  params.set("computerId", computerId);
  if (path) params.set("path", path);

  const response = await fetch(`${API_BASE_URL}/file-transfer/list?${params.toString()}`);
  const json = await parseJSONOrThrow(response);
  if (!json.success) throw new Error(json.error || "Failed to list files.");
  return json.data;
}

async function downloadTransferPath(computerId, path) {
  const params = new URLSearchParams();
  params.set("computerId", computerId);
  params.set("path", path);

  const response = await fetch(`${API_BASE_URL}/file-transfer/download?${params.toString()}`);
  if (!response.ok) {
    let message = "Download failed.";
    try {
      const json = await response.json();
      if (json && json.error) message = json.error;
    } catch (error) {
      // Ignore parse errors and keep fallback message.
    }
    throw new Error(message);
  }

  const disposition = response.headers.get("content-disposition") || "";
  const match = disposition.match(/filename="?([^"]+)"?/i);
  const filename = match ? match[1] : "download.bin";
  const blob = await response.blob();
  return { blob, filename };
}

async function copyTransferPath(
  sourceComputerId,
  sourcePath,
  targetComputerId,
  targetPath = "",
  mode = "copy"
) {
  const response = await fetch(`${API_BASE_URL}/file-transfer/copy`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      sourceComputerId,
      sourcePath,
      targetComputerId,
      targetPath,
      mode,
    }),
  });

  const json = await parseJSONOrThrow(response);
  if (!json.success) throw new Error(json.error || "Copy failed.");
  return json.data;
}

async function undoLastMerge(computerId) {
  const response = await fetch(`${API_BASE_URL}/file-transfer/undo`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ computerId }),
  });

  const json = await parseJSONOrThrow(response);
  if (!json.success) throw new Error(json.error || "Undo failed.");
  return json.data;
}

function toNumber(value) {
  const n = Number(value);
  return Number.isFinite(n) ? n : 0;
}

function parseColonLines(text) {
  const map = {};
  String(text || "")
    .split(/\r?\n/)
    .forEach((line) => {
      const idx = line.indexOf(":");
      if (idx <= 0) return;
      const key = line.slice(0, idx).trim();
      const value = line.slice(idx + 1).trim();
      if (!key) return;
      map[key] = value;
    });
  return map;
}

function parseOsRelease(text) {
  const lines = String(text || "").split(/\r?\n/);
  for (const line of lines) {
    if (line.startsWith("PRETTY_NAME=")) {
      return line.slice("PRETTY_NAME=".length).replace(/^"/, "").replace(/"$/, "").trim();
    }
  }
  return "";
}

function parseSizeToBytes(value) {
  const raw = String(value || "").trim();
  if (!raw || raw === "-") return 0;
  if (raw === "0" || raw === "0B") return 0;

  const match = raw.match(/^([0-9]+(?:\.[0-9]+)?)\s*([kmgtp]?i?b?)?$/i);
  if (!match) {
    const plain = Number(raw);
    return Number.isFinite(plain) ? plain : 0;
  }

  const num = Number(match[1]);
  if (!Number.isFinite(num)) return 0;

  const unit = (match[2] || "b").toLowerCase();
  const powers = {
    b: 0,
    k: 1, kb: 1, kib: 1,
    m: 2, mb: 2, mib: 2,
    g: 3, gb: 3, gib: 3,
    t: 4, tb: 4, tib: 4,
    p: 5, pb: 5, pib: 5,
  };
  const power = powers[unit] ?? 0;
  return num * (1024 ** power);
}

function parseFreeOutputToBytes(text) {
  const lines = String(text || "").split(/\r?\n/);
  const memLine = lines.find((line) => line.trim().startsWith("Mem:"));
  if (!memLine) {
    return { totalBytes: 0, usedBytes: 0, freeBytes: 0 };
  }

  const cols = memLine.trim().split(/\s+/);
  if (cols.length < 4) {
    return { totalBytes: 0, usedBytes: 0, freeBytes: 0 };
  }

  const totalBytes = parseSizeToBytes(cols[1]);
  const usedBytes = parseSizeToBytes(cols[2]);
  const freeBytes = parseSizeToBytes(cols[3]);
  return { totalBytes, usedBytes, freeBytes };
}

function parseDFToBytes(text) {
  const lines = String(text || "")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);

  const dataLine = lines[lines.length - 1] || "";
  const cols = dataLine.split(/\s+/);
  if (cols.length < 5) {
    return { totalBytes: 0, usedBytes: 0, freeBytes: 0, usedPercent: 0 };
  }

  const totalBytes = parseSizeToBytes(cols[1]);
  const usedBytes = parseSizeToBytes(cols[2]);
  const freeBytes = parseSizeToBytes(cols[3]);
  const usedPercent = toNumber(String(cols[4]).replace("%", ""));

  return { totalBytes, usedBytes, freeBytes, usedPercent };
}

function parseLoadValues(text) {
  const parts = String(text || "").trim().split(/\s+/);
  return {
    load1: toNumber(parts[0]),
    load5: toNumber(parts[1]),
    load15: toNumber(parts[2]),
  };
}

function hasUsefulOverview(info) {
  if (!info) return false;
  if (toNumber(info.memoryTotalGB) > 0) return true;
  if (toNumber(info.diskTotalGB) > 0) return true;
  if (info.cpuModel && !String(info.cpuModel).toLowerCase().includes("unknown")) return true;
  return false;
}

async function loadSystemInfoFallbackViaTerminal(computer) {
  const [
    osReleaseRaw,
    lscpuRaw,
    freeRaw,
    dfRaw,
    loadRaw,
    uptimeRaw,
    kernelRaw,
  ] = await Promise.all([
    runTerminalCommand(computer.id, "cat /etc/os-release 2>/dev/null || uname -s"),
    runTerminalCommand(computer.id, "lscpu 2>&1"),
    runTerminalCommand(computer.id, "free -b 2>&1"),
    runTerminalCommand(computer.id, "df -B1 / 2>&1 | tail -n 1"),
    runTerminalCommand(computer.id, "awk '{print $1\" \"$2\" \"$3}' /proc/loadavg 2>/dev/null"),
    runTerminalCommand(computer.id, "uptime -p 2>/dev/null || uptime 2>/dev/null"),
    runTerminalCommand(computer.id, "uname -r 2>/dev/null"),
  ]);

  const osName = parseOsRelease(osReleaseRaw) || String(osReleaseRaw || "").trim() || computer.os || "Unknown Linux";
  const lscpuMap = parseColonLines(lscpuRaw);
  const { totalBytes: memTotalBytes, usedBytes: memUsedBytes } = parseFreeOutputToBytes(freeRaw);
  const {
    totalBytes: diskTotalBytes,
    usedBytes: diskUsedBytes,
    freeBytes: diskFreeBytes,
    usedPercent: diskUsedPercent,
  } = parseDFToBytes(dfRaw);
  const { load1, load5, load15 } = parseLoadValues(loadRaw);

  const coreCount = Math.max(1, Math.trunc(toNumber(lscpuMap["CPU(s)"])));
  const threadsPerCore = Math.max(0, Math.trunc(toNumber(lscpuMap["Thread(s) per core"])));
  const socketCount = Math.max(0, Math.trunc(toNumber(lscpuMap["Socket(s)"])));
  const cpuUsagePercent = clampPercent((load1 / coreCount) * 100);
  const memoryUsagePercent = memTotalBytes > 0 ? (memUsedBytes / memTotalBytes) * 100 : 0;
  const diskUsagePercent = diskUsedPercent || (diskTotalBytes > 0 ? (diskUsedBytes / diskTotalBytes) * 100 : 0);
  const bytesPerGB = 1024 ** 3;

  return {
    computerId: computer.id,
    place: computer.place,
    username: computer.username,
    ip: computer.ip,
    os: osName,
    kernel: String(kernelRaw || "").trim(),
    uptime: String(uptimeRaw || "").trim(),
    architecture: lscpuMap.Architecture || "",
    cpuModel: lscpuMap["Model name"] || "",
    coreCount,
    threadsPerCore,
    socketCount,
    load1: roundTo(load1, 2),
    load5: roundTo(load5, 2),
    load15: roundTo(load15, 2),
    cpuUsagePercent: roundTo(cpuUsagePercent, 2),
    memoryTotalGB: roundTo(memTotalBytes / bytesPerGB, 2),
    memoryUsedGB: roundTo(memUsedBytes / bytesPerGB, 2),
    memoryUsagePercent: roundTo(memoryUsagePercent, 2),
    diskTotalGB: roundTo(diskTotalBytes / bytesPerGB, 2),
    diskUsedGB: roundTo(diskUsedBytes / bytesPerGB, 2),
    diskFreeGB: roundTo(diskFreeBytes / bytesPerGB, 2),
    diskUsagePercent: roundTo(diskUsagePercent, 2),
    collectedAt: new Date().toLocaleString(),
  };
}

function roundTo(value, digits) {
  const factor = 10 ** digits;
  return Math.round((Number(value) || 0) * factor) / factor;
}

/**
 * Loads CPU overview + memory/storage/OS info for one computer via SSH.
 * @param {Object} computer - Computer object
 * @returns {Promise<Object>} CPU overview payload
 */
async function loadSystemInfo(computer) {
  try {
    const response = await fetch(`${API_BASE_URL}/cpu-overview/${computer.id}`);
    const raw = await response.text();
    let json;
    try {
      json = JSON.parse(raw);
    } catch (parseError) {
      // Old backend or HTML fallback: use terminal API fallback.
      return await loadSystemInfoFallbackViaTerminal(computer);
    }
    if (!json.success) throw new Error(json.error);
    if (!hasUsefulOverview(json.data)) {
      return await loadSystemInfoFallbackViaTerminal(computer);
    }
    return json.data;
  } catch (error) {
    // If new endpoint is missing/invalid, fallback to terminal command strategy.
    try {
      return await loadSystemInfoFallbackViaTerminal(computer);
    } catch (fallbackError) {
      console.error(`Failed to load system info for ${computer.id}:`, error, fallbackError);
      throw fallbackError;
    }
  }
}

/**
 * Runs a single SSH command on all computers via the HTTP terminal API.
 * This is a one-shot, non-interactive broadcast — similar to "Check All".
 */
async function runCommandOnAllComputers(command) {
  const logEl = document.getElementById("multi-terminal-log");
  logEl.innerHTML = "";

  if (!command || !command.trim()) {
    showToast("Please enter a command to run.", "error");
    return;
  }

  const runButton = document.getElementById("multi-terminal-run");
  const runButtonOriginalHTML = runButton.innerHTML;
  runButton.disabled = true;
  runButton.textContent = "Running...";

  const appendLine = (text, className = "") => {
    const line = document.createElement("div");
    line.className = `multi-terminal__log-line ${className}`.trim();
    line.textContent = text;
    logEl.appendChild(line);
    logEl.scrollTop = logEl.scrollHeight;
  };

  appendLine(`$ ${command}`, "multi-terminal__log-line--command");

  try {
    const tasks = computers.map(async (c) => {
      try {
        const response = await fetch(`${API_BASE_URL}/terminal/execute`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            computerId: c.id,
            command: command,
          }),
        });
        const json = await parseJSONOrThrow(response);
        const header = `[${c.place} | ${c.id}]`;
        if (!json.success) {
          appendLine(`${header} ERROR: ${json.error || "Unknown error"}`, "multi-terminal__log-line--error");
        } else {
          const out = (json.data && json.data.output) || "";
          const outputLines = out.split(/\r?\n/);
          appendLine(`${header} OK`, "multi-terminal__log-line--success");
          outputLines.forEach((line) => {
            if (line) appendLine(`  ${line}`);
          });
        }
      } catch (err) {
        const header = `[${c.place} | ${c.id}]`;
        appendLine(
          `${header} ERROR: ${err.message || String(err)}`,
          "multi-terminal__log-line--error"
        );
      }
    });

    await Promise.all(tasks);
  } finally {
    runButton.disabled = false;
    runButton.innerHTML = runButtonOriginalHTML;
  }
}

/**
 * Adds a new computer to the monitoring system
 * @param {Object} computer - Computer object with id, name, and ip
 * @returns {Promise<Object>} Created Computer object
 * @throws {Error} If API call fails or validation error
 */
async function addComputer(computer) {
  try {
    const response = await fetch(`${API_BASE_URL}/computers`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(computer),
    });
    const json = await parseJSONOrThrow(response);
    if (!json.success) throw new Error(json.error);
    return json.data;
  } catch (error) {
    console.error("Failed to add computer:", error);
    throw error;
  }
}

/**
 * Updates an existing computer's information
 * @param {string} id - Computer ID to update
 * @param {Object} computer - Computer object with updated name and ip
 * @returns {Promise<Object>} Updated Computer object
 * @throws {Error} If API call fails or validation error
 */
async function updateComputer(id, computer) {
  try {
    const response = await fetch(`${API_BASE_URL}/computers/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(computer),
    });
    const json = await parseJSONOrThrow(response);
    if (!json.success) throw new Error(json.error);
    return json.data;
  } catch (error) {
    console.error(`Failed to update computer ${id}:`, error);
    throw error;
  }
}

/**
 * Deletes a computer from the monitoring system
 * @param {string} id - Computer ID to delete
 * @returns {Promise<Object>} Deleted computer ID
 * @throws {Error} If API call fails
 */
async function deleteComputerAPI(id) {
  try {
    const response = await fetch(`${API_BASE_URL}/computers/${id}`, {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
    });
    const json = await parseJSONOrThrow(response);
    if (!json.success) throw new Error(json.error);
    return json.data;
  } catch (error) {
    console.error(`Failed to delete computer ${id}:`, error);
    throw error;
  }
}

// ============================================================================
// UI Rendering
// ============================================================================

/**
 * Renders or updates a computer card in the grid
 * @param {Object} computer - Computer object to display
 * @param {Object} statusData - Optional ComputerStatus object with status info
 */
function renderCard(computer, statusData = null) {
  const cardElement = document.getElementById(`card-${computer.id}`);
  
  // Determine status information
  const status = statusData ? statusData.status : null;
  const checkedAt = statusData ? statusData.checkedAt : null;
  const osText = computer.os && computer.os.trim() ? computer.os : "Unknown OS";
  
  // Apply appropriate styling based on status
  const statusClass = status === "ON" ? "card--on" : status === "OFF" ? "card--off" : "card--idle";
  const badgeText = status || "UNKNOWN";
  const timeText = checkedAt ? `Checked: ${checkedAt}` : "Not checked yet";

  // Build card HTML
  const cardHTML = `
    <div class="card__indicator"><span class="card__dot"></span></div>
    <div class="card__body">
      <h2 class="card__name">${escapeHTML(computer.place)}</h2>
      <p class="card__username">@${escapeHTML(computer.username)}</p>
      <p class="card__ip">${escapeHTML(computer.ip)}</p>
      <p class="card__os">${escapeHTML(osText)}</p>
    </div>
    <div class="card__status">
      <span class="card__badge">${badgeText}</span>
    </div>
    <div class="card__footer">
      <span class="card__time">${timeText}</span>
      <div class="card__buttons">
        <button class="btn-insights" data-id="${computer.id}" title="CPU overview (lscpu, free -h, dmesg)">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" aria-hidden="true" focusable="false">
            <path d="M3 3v18h18"/>
            <path d="M7 15l3-3 3 2 4-6"/>
          </svg>
        </button>
        <button class="btn-terminal" data-id="${computer.id}" title="Open terminal">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" aria-hidden="true" focusable="false">
            <rect x="3" y="4" width="18" height="14" rx="2" ry="2"/>
            <polyline points="6 14 9 11 6 8"/>
            <line x1="11" y1="14" x2="18" y2="14"/>
          </svg>
        </button>
        <button class="btn-transfer" data-id="${computer.id}" title="Browse files / transfer">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" aria-hidden="true" focusable="false">
            <path d="M3 7h7l2 2h9v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>
            <path d="M13 13h7"/>
            <path d="M17 10l3 3-3 3"/>
          </svg>
        </button>
        <button class="btn-check" data-id="${computer.id}" title="Ping this computer">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" aria-hidden="true" focusable="false">
            <path d="M12 20a8 8 0 1 0-8-8"/>
            <polyline points="4 4 4 12 12 12"/>
          </svg>
        </button>
        <button class="btn-edit" data-id="${computer.id}" title="Edit computer">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" aria-hidden="true" focusable="false">
            <path d="M4 21h4l11-11a2.1 2.1 0 0 0-3-3L5 18l-1 3z"/>
          </svg>
        </button>
        <button class="btn-delete" data-id="${computer.id}" title="Delete computer">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" aria-hidden="true" focusable="false">
            <path d="M3 6h18"/>
            <path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            <path d="M9 10v7"/>
            <path d="M15 10v7"/>
            <rect x="5" y="6" width="14" height="14" rx="2" ry="2"/>
          </svg>
        </button>
      </div>
    </div>
  `;

  // Update existing card or create new one
  if (cardElement) {
    cardElement.className = `card ${statusClass}`;
    cardElement.innerHTML = cardHTML;
  } else {
    const newCard = document.createElement("div");
    newCard.className = `card ${statusClass}`;
    newCard.id = `card-${computer.id}`;
    newCard.innerHTML = cardHTML;
    document.getElementById("grid").appendChild(newCard);
  }

  // Attach event listeners to buttons
  document
    .querySelector(`#card-${computer.id} .btn-insights`)
    .addEventListener("click", () => handleOpenSystemInfo(computer.id));
  document
    .querySelector(`#card-${computer.id} .btn-terminal`)
    .addEventListener("click", () => openTerminalModal(computer));
  document
    .querySelector(`#card-${computer.id} .btn-transfer`)
    .addEventListener("click", () => openFileTransferModal(computer.id));
  document
    .querySelector(`#card-${computer.id} .btn-check`)
    .addEventListener("click", () => handlePingOne(computer.id));
  document
    .querySelector(`#card-${computer.id} .btn-edit`)
    .addEventListener("click", () => openEditModal(computer));
  document
    .querySelector(`#card-${computer.id} .btn-delete`)
    .addEventListener("click", () => handleDeleteComputer(computer.id));
}

/**
 * Sets the loading state of a computer's check button
 * @param {string} id - Computer ID
 * @param {boolean} loading - Whether to show loading state
 */
function setCardLoading(id, loading) {
  const button = document.querySelector(`#card-${id} .btn-check`);
  if (!button) return;

  button.disabled = loading;
  button.innerHTML = loading
    ? `<span class="spinner"></span> Checking...`
    : `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
        <circle cx="12" cy="12" r="10"/>
        <polyline points="12 6 12 12 16 14"/>
      </svg> Check`;
}

function setSystemInfoButtonLoading(id, loading) {
  const button = document.querySelector(`#card-${id} .btn-insights`);
  if (!button) return;

  button.disabled = loading;
  button.innerHTML = loading
    ? `<span class="spinner"></span>`
    : `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" aria-hidden="true" focusable="false">
        <path d="M3 3v18h18"/>
        <path d="M7 15l3-3 3 2 4-6"/>
      </svg>`;
}

function updateCardOSLabel(id, osName) {
  const el = document.querySelector(`#card-${id} .card__os`);
  if (!el) return;
  el.textContent = osName && osName.trim() ? osName : "Unknown OS";
}

/**
 * Updates the summary counts for Total, Online, and Offline computers
 * @param {Array} results - Array of ComputerStatus objects
 */
function updateSummary(results) {
  const onlineCount = results.filter((r) => r.status === "ON").length;
  const offlineCount = results.filter((r) => r.status === "OFF").length;

  document.getElementById("count-total").textContent = computers.length;
  document.getElementById("count-online").textContent = onlineCount;
  document.getElementById("count-offline").textContent = offlineCount;
}

// ============================================================================
// Event Handlers
// ============================================================================

/**
 * Handles pinging a single computer
 * @param {string} id - Computer ID to ping
 */
async function handlePingOne(id) {
  setCardLoading(id, true);
  try {
    const result = await pingOne(id);
    const computer = computers.find((c) => c.id === id);
    if (computer) {
      renderCard(computer, result);
    }
  } catch (error) {
    showToast(`Error checking ${id}: ${error.message}`, "error");
  } finally {
    setCardLoading(id, false);
  }
}

/**
 * Handles pinging all computers at once
 */
async function handlePingAll() {
  const button = document.getElementById("btn-ping-all");
  button.disabled = true;
  button.innerHTML = `<span class="spinner"></span> Checking all...`;

  try {
    const results = await pingAll();
    results.forEach((result) => {
      const computer = computers.find((c) => c.id === result.id);
      if (computer) {
        renderCard(computer, result);
      }
    });
    updateSummary(results);
    showToast("All computers checked!", "success");
  } catch (error) {
    showToast(`Error: ${error.message}`, "error");
  } finally {
    button.disabled = false;
    button.innerHTML = `Check All`;
  }
}

/**
 * Handles adding a new computer from the form
 * @param {Event} event - Form submission event
 */
async function handleAddComputer(event) {
  event.preventDefault();

  const id = document.getElementById("form-id").value.trim();
  const place = document.getElementById("form-place").value.trim();
  const username = document.getElementById("form-user-name").value.trim();
  const ip = document.getElementById("form-ip").value.trim();

  if (!id || !place || !username || !ip) {
    showToast("All fields are required", "error");
    return;
  }

  const submitButton = document.getElementById("form-submit");
  submitButton.disabled = true;
  submitButton.textContent = "Adding...";

  try {
    const newComputer = await addComputer({ id, place, username, ip });
    computers.push(newComputer);
    renderCard(newComputer);
    document.getElementById("count-total").textContent = computers.length;

    document.getElementById("form-id").value = "";
    document.getElementById("form-place").value = "";
    document.getElementById("form-user-name").value = "";
    document.getElementById("form-ip").value = "";

    showToast(`${newComputer.place} added successfully!`, "success");
    closeModal();
  } catch (error) {
    showToast(`Error: ${error.message}`, "error");
  } finally {
    submitButton.disabled = false;
    submitButton.textContent = "Add Computer";
  }
}

/**
 * Handles editing an existing computer
 * @param {Event} event - Form submission event
 */
async function handleEditComputer(event) {
  event.preventDefault();

  const id = document.getElementById("edit-form-id").value.trim();
  const place = document.getElementById("edit-form-place").value.trim();
  const username = document.getElementById("edit-form-user-name").value.trim();
  const ip = document.getElementById("edit-form-ip").value.trim();

  if (!place || !username || !ip) {
    showToast("Place, username, and IP are required", "error");
    return;
  }

  const submitButton = document.getElementById("edit-form-submit");
  submitButton.disabled = true;
  submitButton.textContent = "Updating...";

  try {
    await updateComputer(id, { id, place, username, ip });
    const computerIndex = computers.findIndex((c) => c.id === id);
    if (computerIndex > -1) {
      const existingOS = computers[computerIndex].os || "";
      computers[computerIndex] = { id, place, username, ip, os: existingOS };
      renderCard(computers[computerIndex]);
    }

    showToast(`${place} updated successfully!`, "success");
    closeEditModal();
  } catch (error) {
    showToast(`Error: ${error.message}`, "error");
  } finally {
    submitButton.disabled = false;
    submitButton.textContent = "Update Computer";
  }
}

/**
 * Handles deleting a computer
 * @param {string} id - Computer ID to delete
 */
async function handleDeleteComputer(id) {
  if (!confirm("Are you sure you want to delete this computer?")) {
    return;
  }

  try {
    await deleteComputerAPI(id);
    computers = computers.filter((c) => c.id !== id);
    const card = document.getElementById(`card-${id}`);
    if (card) card.remove();
    document.getElementById("count-total").textContent = computers.length;
    showToast("Computer deleted successfully!", "success");
  } catch (error) {
    showToast(`Error: ${error.message}`, "error");
  }
}

function formatGB(value) {
  return `${Number(value || 0).toFixed(2)} GB`;
}

function clampPercent(value) {
  const num = Number(value || 0);
  if (!Number.isFinite(num)) return 0;
  return Math.max(0, Math.min(100, num));
}

function setUsageBar(barId, percent) {
  const bar = document.getElementById(barId);
  if (!bar) return;
  const safePercent = clampPercent(percent);
  bar.style.width = `${safePercent}%`;
}

function setSystemInfoMessage(text, isError = false) {
  const messageEl = document.getElementById("system-info-message");
  if (!messageEl) return;
  messageEl.textContent = text || "";
  messageEl.classList.toggle("system-info-page__message--error", isError);
}

async function generateSystemReport(info) {
  const memoryTotalGB = Number(info.memoryTotalGB || 0);
  const memoryUsedGB = Number(info.memoryUsedGB || 0);
  const memoryFreeGB = Math.max(memoryTotalGB - memoryUsedGB, 0);

  const diskTotalGB = Number(info.diskTotalGB || 0);
  const diskUsedGB = Number(info.diskUsedGB || 0);
  const diskFreeGB = Number.isFinite(Number(info.diskFreeGB))
    ? Math.max(Number(info.diskFreeGB), 0)
    : Math.max(diskTotalGB - diskUsedGB, 0);

  const cpuPercent = clampPercent(info.cpuUsagePercent);
  const ramPercent = clampPercent(info.memoryUsagePercent);
  const storagePercent = clampPercent(info.diskUsagePercent);
  const worstPercent = Math.max(cpuPercent, ramPercent, storagePercent);

  let health = "Healthy";
  if (worstPercent >= 90) {
    health = "Critical";
  } else if (worstPercent >= 75) {
    health = "Warning";
  }

  return {
    summary: `${info.place}: ${health} utilization profile based on CPU, RAM, and storage.`,
    os: `${info.os || "Unknown OS"}${info.kernel ? ` (${info.kernel})` : ""}`,
    cpu: `${info.cpuModel || "Unknown CPU"} | ${info.coreCount || 0} cores, ${info.threadsPerCore || 0} threads/core, ${info.socketCount || 0} socket(s), ${info.architecture || "Unknown arch"}`,
    ram: `${formatGB(memoryUsedGB)} used, ${formatGB(memoryFreeGB)} free of ${formatGB(memoryTotalGB)} (${ramPercent.toFixed(1)}%)`,
    storage: `${formatGB(diskUsedGB)} used, ${formatGB(diskFreeGB)} free of ${formatGB(diskTotalGB)} (${storagePercent.toFixed(1)}%)`,
    health: `${health} | Load ${Number(info.load1 || 0).toFixed(2)} / ${Number(info.load5 || 0).toFixed(2)} / ${Number(info.load15 || 0).toFixed(2)} | Uptime: ${info.uptime || "-"}`,
    memoryFreeGB,
    diskFreeGB,
  };
}

function renderSystemInfoLoading(computer) {
  document.getElementById("system-info-title").textContent = `CPU Overview - ${computer.place}`;
  document.getElementById("system-info-subtitle").textContent = `${computer.username}@${computer.ip}`;
  document.getElementById("system-info-os").textContent = computer.os || "Detecting...";
  document.getElementById("system-info-cpu-model").textContent = "Detecting...";
  document.getElementById("system-info-cpu-arch").textContent = "Detecting...";
  document.getElementById("system-info-updated").textContent = "Fetching latest data...";
  document.getElementById("system-cpu-percent").textContent = "--%";
  document.getElementById("system-memory-percent").textContent = "--%";
  document.getElementById("system-disk-percent").textContent = "--%";
  document.getElementById("system-cpu-text").textContent = "--";
  document.getElementById("system-memory-text").textContent = "-- / --";
  document.getElementById("system-disk-text").textContent = "-- / --";
  document.getElementById("system-report-summary").textContent = "Analyzing host metrics...";
  document.getElementById("system-report-os").textContent = "OS: ...";
  document.getElementById("system-report-cpu").textContent = "CPU: ...";
  document.getElementById("system-report-ram").textContent = "RAM: ...";
  document.getElementById("system-report-storage").textContent = "Storage: ...";
  document.getElementById("system-report-health").textContent = "Health: ...";
  setUsageBar("system-cpu-bar", 0);
  setUsageBar("system-memory-bar", 0);
  setUsageBar("system-disk-bar", 0);
  setSystemInfoMessage("Running SSH checks for CPU, RAM, storage, and OS...");
}

function renderSystemInfoData(info, report) {
  document.getElementById("system-info-title").textContent = `CPU Overview - ${info.place}`;
  document.getElementById("system-info-subtitle").textContent = `${info.username}@${info.ip}`;
  document.getElementById("system-info-os").textContent = info.os || "Unknown OS";
  document.getElementById("system-info-cpu-model").textContent = info.cpuModel || "Unknown CPU";
  const coreText = `${info.coreCount || 0} cores • ${info.threadsPerCore || 0} threads/core • ${info.architecture || "Unknown arch"}`;
  document.getElementById("system-info-cpu-arch").textContent = coreText;
  document.getElementById("system-info-updated").textContent = info.collectedAt || "-";

  const cpuPercent = clampPercent(info.cpuUsagePercent);
  const memoryPercent = clampPercent(info.memoryUsagePercent);
  const diskPercent = clampPercent(info.diskUsagePercent);

  document.getElementById("system-cpu-percent").textContent = `${cpuPercent.toFixed(1)}%`;
  document.getElementById("system-memory-percent").textContent = `${memoryPercent.toFixed(1)}%`;
  document.getElementById("system-disk-percent").textContent = `${diskPercent.toFixed(1)}%`;
  document.getElementById("system-cpu-text").textContent =
    `Load: ${Number(info.load1 || 0).toFixed(2)} / ${Number(info.load5 || 0).toFixed(2)} / ${Number(info.load15 || 0).toFixed(2)}  |  Uptime: ${info.uptime || "-"}`;
  document.getElementById("system-memory-text").textContent =
    `${formatGB(info.memoryUsedGB)} used, ${formatGB(report.memoryFreeGB)} free of ${formatGB(info.memoryTotalGB)}`;
  document.getElementById("system-disk-text").textContent =
    `${formatGB(info.diskUsedGB)} used, ${formatGB(report.diskFreeGB)} free of ${formatGB(info.diskTotalGB)}`;

  document.getElementById("system-report-summary").textContent = report.summary;
  document.getElementById("system-report-os").textContent = `OS: ${report.os}`;
  document.getElementById("system-report-cpu").textContent = `CPU: ${report.cpu}`;
  document.getElementById("system-report-ram").textContent = `RAM: ${report.ram}`;
  document.getElementById("system-report-storage").textContent = `Storage: ${report.storage}`;
  document.getElementById("system-report-health").textContent = `Health: ${report.health}`;

  setUsageBar("system-cpu-bar", cpuPercent);
  setUsageBar("system-memory-bar", memoryPercent);
  setUsageBar("system-disk-bar", diskPercent);
  setSystemInfoMessage("");
}

function renderSystemInfoError(errorMessage) {
  setSystemInfoMessage(errorMessage || "Failed to load system info.", true);
  document.getElementById("system-report-summary").textContent = "Report generation failed.";
  document.getElementById("system-report-health").textContent = "Health: unavailable";
}

function openSystemInfoPage(computer) {
  const page = document.getElementById("system-info-page");
  page.classList.add("active");
  document.body.style.overflow = "hidden";
  renderSystemInfoLoading(computer);
}

function closeSystemInfoPage() {
  const page = document.getElementById("system-info-page");
  page.classList.remove("active");
  if (!document.getElementById("terminal-modal").classList.contains("active")) {
    document.body.style.overflow = "auto";
  }
}

async function handleOpenSystemInfo(id) {
  const computer = computers.find((c) => c.id === id);
  if (!computer) {
    showToast("Computer not found.", "error");
    return;
  }

  openSystemInfoPage(computer);
  setSystemInfoButtonLoading(id, true);

  try {
    const info = await loadSystemInfo(computer);
    const report = await generateSystemReport(info);
    renderSystemInfoData(info, report);

    const target = computers.find((c) => c.id === id);
    if (target) {
      target.os = info.os || "";
      updateCardOSLabel(id, target.os);
    }
  } catch (error) {
    renderSystemInfoError(error.message);
    showToast(`System info error: ${error.message}`, "error");
  } finally {
    setSystemInfoButtonLoading(id, false);
  }
}

// ============================================================================
// Terminal Management
// ============================================================================

let currentTerminalComputer = null;
let terminalSocket = null;
let terminalInstance = null;
let terminalFallbackMode = false; // true = plain text div instead of xterm
const terminalCommandHistory = [];
const terminalHistoryLimit = 100;
let terminalHistoryIndex = -1; // -1 means not currently browsing history
let terminalHistoryDraft = "";

// Strip ANSI/control escape sequences when xterm.js is unavailable.
function stripAnsiControlCodes(text) {
  return text
    // OSC sequences (window title, etc.)
    .replace(/\x1B\][^\x07]*(?:\x07|\x1B\\)/g, "")
    // CSI sequences (colors, cursor movement, bracketed paste mode)
    .replace(/\x1B\[[0-?]*[ -/]*[@-~]/g, "")
    // Single-character escape sequences
    .replace(/\x1B[@-Z\\-_]/g, "")
    // Bell
    .replace(/\x07/g, "")
    // Carriage returns create noisy artifacts in plain div mode
    .replace(/\r/g, "");
}

function normalizeTerminalChunks(rawText) {
  return stripAnsiControlCodes(rawText)
    .split(/\n/)
    .map((line) => line.trimEnd())
    .filter((line) => line);
}

function setInputCaretToEnd(inputEl) {
  const len = inputEl.value.length;
  inputEl.setSelectionRange(len, len);
}

function rememberTerminalCommand(command) {
  if (!command) return;

  if (terminalCommandHistory.length === 0 || terminalCommandHistory[terminalCommandHistory.length - 1] !== command) {
    terminalCommandHistory.push(command);
    if (terminalCommandHistory.length > terminalHistoryLimit) {
      terminalCommandHistory.shift();
    }
  }

  terminalHistoryIndex = -1;
  terminalHistoryDraft = "";
}

function navigateTerminalHistory(direction) {
  const inputEl = document.getElementById("terminal-input");
  if (!inputEl || terminalCommandHistory.length === 0) return;

  if (direction < 0) {
    if (terminalHistoryIndex === -1) {
      terminalHistoryDraft = inputEl.value;
      terminalHistoryIndex = terminalCommandHistory.length - 1;
    } else if (terminalHistoryIndex > 0) {
      terminalHistoryIndex -= 1;
    }

    inputEl.value = terminalCommandHistory[terminalHistoryIndex];
    setInputCaretToEnd(inputEl);
    return;
  }

  if (terminalHistoryIndex === -1) return;

  if (terminalHistoryIndex < terminalCommandHistory.length - 1) {
    terminalHistoryIndex += 1;
    inputEl.value = terminalCommandHistory[terminalHistoryIndex];
  } else {
    terminalHistoryIndex = -1;
    inputEl.value = terminalHistoryDraft;
  }

  setInputCaretToEnd(inputEl);
}

/**
 * Sends a single command from the bottom input box into the SSH terminal.
 * Mirrors what would happen if you typed directly in xterm.
 */
function sendTerminalCommandFromInput() {
  const inputEl = document.getElementById("terminal-input");
  if (!inputEl) return;

  const raw = inputEl.value;
  const command = raw.trimEnd();
  if (!command) return;

  rememberTerminalCommand(command);

  // Echo the command into the terminal for clarity
  if (terminalInstance) {
    terminalInstance.write("\r\n\x1b[1;36m$ " + command + "\x1b[0m\r\n");
  }

  if (terminalSocket && terminalSocket.readyState === WebSocket.OPEN) {
    // Send the command plus newline to the SSH shell
    terminalSocket.send(command + "\n");
  }

  inputEl.value = "";
}

/**
 * Opens the terminal modal and connects via WebSocket + xterm.js
 * @param {Object} computer - Computer object to connect to
 */
function openTerminalModal(computer) {
  currentTerminalComputer = computer;

  document.getElementById("terminal-title").textContent =
    `${computer.place} — ${computer.username}@${computer.ip}`;

  document.getElementById("terminal-modal").classList.add("active");
  document.body.style.overflow = "hidden";

  const outputEl = document.getElementById("terminal-output");
  outputEl.innerHTML = "";

  // If xterm is not available (CDN blocked / offline), use a simple text fallback
  if (!window.Terminal || !window.FitAddon) {
    terminalFallbackMode = true;
    initPlainTerminal(computer, outputEl);
  } else {
    terminalFallbackMode = false;
    initXterm(computer, outputEl);
  }
}

// Very simple plain-text terminal using a div + WebSocket (no xterm.js needed)
function initPlainTerminal(computer, container) {
  if (terminalSocket) { terminalSocket.close(); terminalSocket = null; }

  const statusEl = document.getElementById("terminal-status");
  const inputEl = document.getElementById("terminal-input");

  const appendLine = (text) => {
    const line = document.createElement("div");
    line.className = "terminal-line__output";
    line.textContent = text;
    container.appendChild(line);
    container.scrollTop = container.scrollHeight;
  };

  appendLine(`Connecting to ${computer.username}@${computer.ip}...`);
  statusEl.textContent = "Connecting...";

  const wsURL = `ws://localhost:8081/api/terminal/ws?computerId=${encodeURIComponent(computer.id)}`;
  terminalSocket = new WebSocket(wsURL);
  // Ensure stdout/stderr binary frames arrive as ArrayBuffer, not Blob
  terminalSocket.binaryType = "arraybuffer";

  terminalSocket.onopen = () => {
    statusEl.textContent = "Connected";
    appendLine("Connected. Type commands below; output will appear here.");
  };

  terminalSocket.onmessage = (event) => {
    const rawText =
      typeof event.data === "string"
        ? event.data
        : new TextDecoder().decode(event.data);

    normalizeTerminalChunks(rawText).forEach((chunk) => appendLine(chunk));
  };

  terminalSocket.onerror = () => {
    statusEl.textContent = "Error";
    appendLine("WebSocket error — is the server running?");
  };

  terminalSocket.onclose = () => {
    statusEl.textContent = "Disconnected";
    appendLine("Connection closed.");
  };

  // For fallback mode, Enter in the bottom input sends command lines
  if (inputEl) {
    inputEl.focus();
  }
}

function initXterm(computer, container) {
  if (terminalInstance) { terminalInstance.dispose(); terminalInstance = null; }
  if (terminalSocket) { terminalSocket.close(); terminalSocket = null; }

  terminalInstance = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: "'JetBrains Mono', 'Cascadia Code', 'Fira Code', monospace",
    theme: {
      background: "#050812",
      foreground: "#c9d1d9",
      cursor: "#6366f1",
      selectionBackground: "#264f78",
      black: "#050812",
      green: "#10b981",
      yellow: "#d29922",
      red: "#ef4444",
      blue: "#6366f1",
      cyan: "#39c5cf",
    },
    scrollback: 5000,
    convertEol: true,
    allowTransparency: false,
  });

  // FitAddon makes the terminal fill the container correctly
  const fitAddon = new window.FitAddon.FitAddon();
  terminalInstance.loadAddon(fitAddon);

  terminalInstance.open(container);

  // fit() MUST be called after open() so the canvas gets correct dimensions
  fitAddon.fit();

  terminalInstance.write("\x1b[1;34mConnecting to " + computer.username + "@" + computer.ip + "...\x1b[0m\r\n");
  document.getElementById("terminal-status").textContent = "Connecting...";

  const wsURL = `ws://localhost:8081/api/terminal/ws?computerId=${encodeURIComponent(computer.id)}`;
  terminalSocket = new WebSocket(wsURL);
  terminalSocket.binaryType = "arraybuffer";

  terminalSocket.onopen = () => {
    document.getElementById("terminal-status").textContent = "Connected";
    terminalInstance.write("\x1b[1;32mConnected!\x1b[0m\r\n");
    fitAddon.fit(); // refit after modal is fully visible
  };

  terminalSocket.onmessage = (event) => {
    if (event.data instanceof ArrayBuffer) {
      terminalInstance.write(new Uint8Array(event.data));
    } else {
      terminalInstance.write(event.data);
    }
  };

  terminalSocket.onerror = () => {
    terminalInstance.write("\x1b[1;31mWebSocket error — is the server running?\x1b[0m\r\n");
    document.getElementById("terminal-status").textContent = "Error";
  };

  terminalSocket.onclose = () => {
    terminalInstance.write("\r\n\x1b[1;33mConnection closed.\x1b[0m\r\n");
    document.getElementById("terminal-status").textContent = "Disconnected";
  };

  terminalInstance.onData((data) => {
    if (terminalSocket && terminalSocket.readyState === WebSocket.OPEN) {
      terminalSocket.send(data);
    }
  });

  terminalInstance.onResize(({ cols, rows }) => {
    if (terminalSocket && terminalSocket.readyState === WebSocket.OPEN) {
      terminalSocket.send(JSON.stringify({ type: "resize", cols, rows }));
    }
  });

  // Refit on window resize so terminal always fills the modal
  window._termResizeHandler = () => fitAddon.fit();
  window.addEventListener("resize", window._termResizeHandler);

  // focus AFTER fit, with small delay to let modal finish rendering
  setTimeout(() => { fitAddon.fit(); terminalInstance.focus(); }, 150);
}

/**
 * Closes the terminal modal and cleans up WebSocket + xterm
 */
function closeTerminalModal() {
  document.getElementById("terminal-modal").classList.remove("active");
  if (!document.getElementById("system-info-page").classList.contains("active")) {
    document.body.style.overflow = "auto";
  }

  if (window._termResizeHandler) {
    window.removeEventListener("resize", window._termResizeHandler);
    window._termResizeHandler = null;
  }
  if (terminalSocket) { terminalSocket.close(); terminalSocket = null; }
  if (terminalInstance) { terminalInstance.dispose(); terminalInstance = null; }
  currentTerminalComputer = null;
}

// ============================================================================
// File Transfer Modal
// ============================================================================

function formatBytes(value) {
  const bytes = Number(value || 0);
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let idx = 0;
  let size = bytes;
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024;
    idx += 1;
  }
  return `${size.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
}

function getComputerLabelById(id) {
  const computer = computers.find((c) => c.id === id);
  if (!computer) return id || "Unknown";
  return `${computer.place} (${computer.username}@${computer.ip})`;
}

function renderFileTransferRoots() {
  const rootsEl = document.getElementById("file-transfer-roots");
  if (!rootsEl) return;

  const roots = Array.isArray(fileTransferState.roots) ? fileTransferState.roots : [];
  rootsEl.innerHTML = "";

  roots.forEach((root) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "file-transfer__root-btn";
    if (!root.exists) button.classList.add("file-transfer__root-btn--disabled");
    if (root.path === fileTransferState.currentPath) button.classList.add("file-transfer__root-btn--active");
    button.textContent = root.label;
    button.disabled = !root.exists;
    button.addEventListener("click", () => refreshFileTransferList(root.path));
    rootsEl.appendChild(button);
  });
}

function selectFileTransferPath(path) {
  fileTransferState.selectedPath = String(path || "").trim();
  const selectedEl = document.getElementById("file-transfer-selected");
  if (!selectedEl) return;
  selectedEl.textContent = fileTransferState.selectedPath || "None";
}

function renderFileTransferEntries() {
  const listEl = document.getElementById("file-transfer-list");
  if (!listEl) return;

  const entries = Array.isArray(fileTransferState.entries) ? fileTransferState.entries : [];
  if (entries.length === 0) {
    listEl.innerHTML = `<p class="file-transfer__empty">No files or folders found in this directory.</p>`;
    return;
  }

  listEl.innerHTML = "";
  entries.forEach((entry) => {
    const row = document.createElement("div");
    row.className = "file-transfer__entry";

    const nameButton = document.createElement("button");
    nameButton.type = "button";
    nameButton.className = "file-transfer__entry-main";
    nameButton.innerHTML = `
      <span class="file-transfer__entry-kind">${entry.isDir ? "DIR" : "FILE"}</span>
      <span class="file-transfer__entry-name">${escapeHTML(entry.name)}</span>
      <span class="file-transfer__entry-meta">${entry.isDir ? "Folder" : formatBytes(entry.sizeBytes)} • ${escapeHTML(entry.modifiedAt || "-")}</span>
    `;
    if (entry.isDir) {
      nameButton.addEventListener("click", () => refreshFileTransferList(entry.path));
    } else {
      nameButton.addEventListener("click", () => selectFileTransferPath(entry.path));
    }

    const actions = document.createElement("div");
    actions.className = "file-transfer__entry-actions";

    if (entry.isDir) {
      const openButton = document.createElement("button");
      openButton.type = "button";
      openButton.className = "file-transfer__mini-btn";
      openButton.textContent = "Open";
      openButton.addEventListener("click", () => refreshFileTransferList(entry.path));
      actions.appendChild(openButton);
    }

    const selectButton = document.createElement("button");
    selectButton.type = "button";
    selectButton.className = "file-transfer__mini-btn file-transfer__mini-btn--select";
    if (entry.path === fileTransferState.selectedPath) {
      selectButton.classList.add("file-transfer__mini-btn--active");
    }
    selectButton.textContent = "Select";
    selectButton.addEventListener("click", () => {
      selectFileTransferPath(entry.path);
      renderFileTransferEntries();
    });
    actions.appendChild(selectButton);

    row.appendChild(nameButton);
    row.appendChild(actions);
    listEl.appendChild(row);
  });
}

async function refreshFileTransferList(path = "") {
  const sourceId = fileTransferState.sourceComputerId;
  if (!sourceId) return;

  const listEl = document.getElementById("file-transfer-list");
  if (listEl) {
    listEl.innerHTML = `<p class="file-transfer__loading">Loading files from ${escapeHTML(getComputerLabelById(sourceId))}...</p>`;
  }

  try {
    const data = await listTransferFiles(sourceId, path);
    fileTransferState.currentPath = data.currentPath || "";
    fileTransferState.parentPath = data.parentPath || "";
    fileTransferState.homePath = data.homePath || "";
    fileTransferState.roots = Array.isArray(data.roots) ? data.roots : [];
    fileTransferState.entries = Array.isArray(data.entries) ? data.entries : [];

    if (fileTransferState.selectedPath && !fileTransferState.selectedPath.startsWith(fileTransferState.homePath)) {
      fileTransferState.selectedPath = "";
    }

    const pathEl = document.getElementById("file-transfer-path");
    if (pathEl) pathEl.textContent = fileTransferState.currentPath || "-";

    const upButton = document.getElementById("file-transfer-up");
    if (upButton) upButton.disabled = !fileTransferState.parentPath;

    renderFileTransferRoots();
    renderFileTransferEntries();
    selectFileTransferPath(fileTransferState.selectedPath);
  } catch (error) {
    if (listEl) {
      listEl.innerHTML = `<p class="file-transfer__error">${escapeHTML(error.message || "Failed to load files.")}</p>`;
    }
    showToast(`File list error: ${error.message}`, "error");
  }
}

function populateFileTransferComputers(initialSourceId = "") {
  const sourceSelect = document.getElementById("file-transfer-source");
  const targetSelect = document.getElementById("file-transfer-target");
  if (!sourceSelect || !targetSelect) return;

  const sourceId = initialSourceId || computers[0]?.id || "";
  const targetFallback = computers.find((c) => c.id !== sourceId)?.id || sourceId;

  sourceSelect.innerHTML = "";
  targetSelect.innerHTML = "";

  computers.forEach((computer) => {
    const sourceOption = document.createElement("option");
    sourceOption.value = computer.id;
    sourceOption.textContent = `${computer.place} (${computer.username}@${computer.ip})`;
    sourceSelect.appendChild(sourceOption);

    const targetOption = document.createElement("option");
    targetOption.value = computer.id;
    targetOption.textContent = `${computer.place} (${computer.username}@${computer.ip})`;
    targetSelect.appendChild(targetOption);
  });

  sourceSelect.value = sourceId;
  targetSelect.value = targetFallback;
  fileTransferState.sourceComputerId = sourceSelect.value;
  fileTransferState.targetComputerId = targetSelect.value;
}

function openFileTransferModal(initialSourceId = "") {
  if (computers.length === 0) {
    showToast("No computers configured.", "error");
    return;
  }

  populateFileTransferComputers(initialSourceId);
  fileTransferState.selectedPath = "";
  selectFileTransferPath("");

  document.getElementById("file-transfer-modal").classList.add("modal--open");
  refreshFileTransferList("");
}

function closeFileTransferModal() {
  document.getElementById("file-transfer-modal").classList.remove("modal--open");
}

async function handleFileTransferDownload() {
  if (!fileTransferState.sourceComputerId || !fileTransferState.selectedPath) {
    showToast("Select a file or folder first.", "error");
    return;
  }

  const button = document.getElementById("file-transfer-download");
  button.disabled = true;
  const previousLabel = button.textContent;
  button.textContent = "Downloading...";

  try {
    const { blob, filename } = await downloadTransferPath(
      fileTransferState.sourceComputerId,
      fileTransferState.selectedPath
    );
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = filename || "download.bin";
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);

    showToast("Download started.", "success");
  } catch (error) {
    showToast(`Download failed: ${error.message}`, "error");
  } finally {
    button.disabled = false;
    button.textContent = previousLabel;
  }
}

async function handleFileTransferCopy() {
  const sourceId = fileTransferState.sourceComputerId;
  const targetId = document.getElementById("file-transfer-target")?.value || "";
  const selectedPath = fileTransferState.selectedPath;

  if (!sourceId || !targetId || !selectedPath) {
    showToast("Select source file/folder and target computer first.", "error");
    return;
  }

  if (sourceId === targetId) {
    showToast("Choose a different target computer for paste.", "error");
    return;
  }

  const button = document.getElementById("file-transfer-copy");
  button.disabled = true;
  const previousLabel = button.textContent;
  button.textContent = "Pasting...";

  try {
    await copyTransferPath(sourceId, selectedPath, targetId, "", "copy");
    showToast(`Pasted to ${getComputerLabelById(targetId)} (Downloads/Home).`, "success");
  } catch (error) {
    showToast(`Paste failed: ${error.message}`, "error");
  } finally {
    button.disabled = false;
    button.textContent = previousLabel;
  }
}

async function handleFileTransferMerge() {
  const sourceId = fileTransferState.sourceComputerId;
  const targetId = document.getElementById("file-transfer-target")?.value || "";
  const selectedPath = fileTransferState.selectedPath;

  if (!sourceId || !targetId || !selectedPath) {
    showToast("Select source folder and target computer first.", "error");
    return;
  }

  if (sourceId === targetId) {
    showToast("Choose a different target computer for merge.", "error");
    return;
  }

  // Ensure the selected path is a directory before attempting merge.
  const entries = Array.isArray(fileTransferState.entries) ? fileTransferState.entries : [];
  const entry = entries.find((e) => e.path === selectedPath);
  if (!entry || !entry.isDir) {
    showToast("Merge is only supported for folders. Please select a folder.", "error");
    return;
  }

  const button = document.getElementById("file-transfer-merge");
  button.disabled = true;
  const previousLabel = button.textContent;
  button.textContent = "Merging...";

  try {
    await copyTransferPath(sourceId, selectedPath, targetId, "", "merge_newer");
    showToast(
      `Merged folder into ${getComputerLabelById(targetId)} (Downloads/Home).`,
      "success"
    );
  } catch (error) {
    showToast(`Merge failed: ${error.message}`, "error");
  } finally {
    button.disabled = false;
    button.textContent = previousLabel;
  }
}

async function handleFileTransferUndo() {
  const targetId = document.getElementById("file-transfer-target")?.value || "";
  if (!targetId) {
    showToast("Select a target computer to undo the last merge.", "error");
    return;
  }

  const button = document.getElementById("file-transfer-undo");
  button.disabled = true;
  const previousLabel = button.textContent;
  button.textContent = "Undoing...";

  try {
    await undoLastMerge(targetId);
    showToast(`Undo complete on ${getComputerLabelById(targetId)}.`, "success");
  } catch (error) {
    showToast(`Undo failed: ${error.message}`, "error");
  } finally {
    button.disabled = false;
    button.textContent = previousLabel;
  }
}
// ============================================================================
// Modal Management
// ============================================================================

function openModal() {
  document.getElementById("modal").classList.add("modal--open");
}

function closeModal() {
  document.getElementById("modal").classList.remove("modal--open");
}

function openEditModal(computer) {
  document.getElementById("edit-form-id").value = computer.id;
  document.getElementById("edit-form-place").value = computer.place;
  document.getElementById("edit-form-user-name").value = computer.username;
  document.getElementById("edit-form-ip").value = computer.ip;
  document.getElementById("edit-modal").classList.add("modal--open");
}

function closeEditModal() {
  document.getElementById("edit-modal").classList.remove("modal--open");
}

// ============================================================================
// Notifications
// ============================================================================

function showToast(message, type = "success") {
  const toast = document.getElementById("toast");
  toast.textContent = message;
  toast.className = `toast toast--${type} toast--show`;
  setTimeout(() => toast.classList.remove("toast--show"), 3000);
}

// ============================================================================
// Utilities
// ============================================================================

function escapeHTML(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

// ============================================================================
// Application Initialization
// ============================================================================

async function init() {
  try {
    computers = await loadComputers();
    computers.forEach((c) => renderCard(c));
    document.getElementById("count-total").textContent = computers.length;
    document.getElementById("count-online").textContent = "0";
    document.getElementById("count-offline").textContent = "0";
    console.log(`Loaded ${computers.length} computers successfully`);
  } catch (error) {
    const errorDiv = document.getElementById("error");
    errorDiv.style.display = "block";
    errorDiv.textContent = "⚠️ Cannot reach server. Is the Go server running?";
    console.error("Failed to initialize application:", error);
  }
}

function setupEventListeners() {
  document.getElementById("btn-ping-all").addEventListener("click", handlePingAll);
  document.getElementById("btn-add").addEventListener("click", openModal);
  document.getElementById("btn-file-transfer").addEventListener("click", () => openFileTransferModal());
  document.getElementById("btn-terminal-all").addEventListener("click", () => {
    document.getElementById("multi-terminal-modal").classList.add("modal--open");
    document.getElementById("multi-terminal-command").focus();
    document.getElementById("multi-terminal-log").innerHTML = "";
  });

  // Add modal listeners
  document.getElementById("modal-close").addEventListener("click", closeModal);
  document.getElementById("modal").addEventListener("click", (e) => {
    if (e.target.id === "modal") closeModal();
  });
  document.getElementById("add-form").addEventListener("submit", handleAddComputer);

  // Edit modal listeners
  document.getElementById("edit-modal-close").addEventListener("click", closeEditModal);
  document.getElementById("edit-modal").addEventListener("click", (e) => {
    if (e.target.id === "edit-modal") closeEditModal();
  });
  document.getElementById("edit-form").addEventListener("submit", handleEditComputer);

  // Multi-terminal (run command on all) modal listeners
  document.getElementById("multi-terminal-close").addEventListener("click", () => {
    document.getElementById("multi-terminal-modal").classList.remove("modal--open");
  });
  document.getElementById("multi-terminal-modal").addEventListener("click", (e) => {
    if (e.target.id === "multi-terminal-modal") {
      document.getElementById("multi-terminal-modal").classList.remove("modal--open");
    }
  });
  document.getElementById("multi-terminal-run").addEventListener("click", () => {
    const cmdInput = document.getElementById("multi-terminal-command");
    runCommandOnAllComputers(cmdInput.value);
  });
  document.getElementById("multi-terminal-command").addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      runCommandOnAllComputers(e.target.value);
    }
  });

  // File transfer modal listeners
  document.getElementById("file-transfer-close").addEventListener("click", closeFileTransferModal);
  document.getElementById("file-transfer-modal").addEventListener("click", (e) => {
    if (e.target.id === "file-transfer-modal") closeFileTransferModal();
  });
  document.getElementById("file-transfer-source").addEventListener("change", (e) => {
    fileTransferState.sourceComputerId = e.target.value;
    const targetSelect = document.getElementById("file-transfer-target");
    if (targetSelect && targetSelect.value === fileTransferState.sourceComputerId) {
      const fallback = computers.find((c) => c.id !== fileTransferState.sourceComputerId);
      if (fallback) {
        targetSelect.value = fallback.id;
      }
    }
    fileTransferState.selectedPath = "";
    selectFileTransferPath("");
    refreshFileTransferList("");
  });
  document.getElementById("file-transfer-target").addEventListener("change", (e) => {
    fileTransferState.targetComputerId = e.target.value;
  });
  document.getElementById("file-transfer-up").addEventListener("click", () => {
    if (!fileTransferState.parentPath) return;
    refreshFileTransferList(fileTransferState.parentPath);
  });
  document.getElementById("file-transfer-download").addEventListener("click", handleFileTransferDownload);
  document.getElementById("file-transfer-copy").addEventListener("click", handleFileTransferCopy);
  document.getElementById("file-transfer-merge").addEventListener("click", handleFileTransferMerge);
  document.getElementById("file-transfer-undo").addEventListener("click", handleFileTransferUndo);

  // Terminal modal listeners
  document.getElementById("terminal-modal-close").addEventListener("click", closeTerminalModal);
  document.getElementById("terminal-modal").addEventListener("click", (e) => {
    if (e.target.id === "terminal-modal") closeTerminalModal();
  });

  // System info full-page listeners
  document.getElementById("system-info-close").addEventListener("click", closeSystemInfoPage);
  document.getElementById("system-info-page").addEventListener("click", (e) => {
    if (e.target.id === "system-info-page") closeSystemInfoPage();
  });

  // Terminal command input (bottom bar)
  const terminalInput = document.getElementById("terminal-input");
  const terminalSendBtn = document.getElementById("terminal-send-btn");

  if (terminalInput) {
    terminalInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        sendTerminalCommandFromInput();
        return;
      }

      if (e.key === "ArrowUp") {
        e.preventDefault();
        navigateTerminalHistory(-1);
        return;
      }

      if (e.key === "ArrowDown") {
        e.preventDefault();
        navigateTerminalHistory(1);
      }
    });

    terminalInput.addEventListener("input", () => {
      if (terminalHistoryIndex === -1) {
        terminalHistoryDraft = terminalInput.value;
      }
    });
  }

  if (terminalSendBtn) {
    terminalSendBtn.addEventListener("click", (e) => {
      e.preventDefault();
      sendTerminalCommandFromInput();
    });
  }

  // Global Escape key to close terminal
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      if (document.getElementById("terminal-modal").classList.contains("active")) {
        closeTerminalModal();
        return;
      }
      if (document.getElementById("system-info-page").classList.contains("active")) {
        closeSystemInfoPage();
        return;
      }
      if (document.getElementById("file-transfer-modal").classList.contains("modal--open")) {
        closeFileTransferModal();
      }
    }
  });
}

// ✅ Start application — no duplicate listeners below this line
setupEventListeners();
init();
