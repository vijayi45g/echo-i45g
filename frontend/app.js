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

// ============================================================================
// API Service Layer
// ============================================================================

/**
 * Fetches all computers from the backend API
 * @returns {Promise<Array>} Array of Computer objects
 * @throws {Error} If API call fails
 */
async function loadComputers() {
  try {
    const response = await fetch(`${API_BASE_URL}/computers`);
    const json = await response.json();
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
    const json = await response.json();
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
    const json = await response.json();
    if (!json.success) throw new Error(json.error);
    return json.data;
  } catch (error) {
    console.error("Failed to ping all computers:", error);
    throw error;
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
        const json = await response.json();
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
    runButton.textContent = "";
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
    const json = await response.json();
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
    const json = await response.json();
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
    const json = await response.json();
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
    </div>
    <div class="card__status">
      <span class="card__badge">${badgeText}</span>
    </div>
    <div class="card__footer">
      <span class="card__time">${timeText}</span>
      <div class="card__buttons">
        <button class="btn-terminal" data-id="${computer.id}" title="Open SSH terminal">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" aria-hidden="true" focusable="false">
            <rect x="3" y="4" width="18" height="14" rx="2" ry="2"/>
            <polyline points="6 14 9 11 6 8"/>
            <line x1="11" y1="14" x2="18" y2="14"/>
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
    .querySelector(`#card-${computer.id} .btn-terminal`)
    .addEventListener("click", () => openTerminalModal(computer));
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
      computers[computerIndex] = { id, place, username, ip };
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

// ============================================================================
// Terminal Management
// ============================================================================

let currentTerminalComputer = null;
let terminalSocket = null;
let terminalInstance = null;
let terminalFallbackMode = false; // true = plain text div instead of xterm

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

  appendLine(`Connecting to ${computer.username}@${computer.ip} via SSH...`);
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
    if (typeof event.data === "string") {
      // Backend sends \r\n – normalize to \n for the div
      event.data.split(/\r?\n/).forEach((chunk) => {
        if (chunk) appendLine(chunk);
      });
    } else {
      const text = new TextDecoder().decode(event.data);
      text.split(/\r?\n/).forEach((chunk) => {
        if (chunk) appendLine(chunk);
      });
    }
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
  document.body.style.overflow = "auto";

  if (window._termResizeHandler) {
    window.removeEventListener("resize", window._termResizeHandler);
    window._termResizeHandler = null;
  }
  if (terminalSocket) { terminalSocket.close(); terminalSocket = null; }
  if (terminalInstance) { terminalInstance.dispose(); terminalInstance = null; }
  currentTerminalComputer = null;
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

  // Terminal modal listeners
  document.getElementById("terminal-modal-close").addEventListener("click", closeTerminalModal);
  document.getElementById("terminal-modal").addEventListener("click", (e) => {
    if (e.target.id === "terminal-modal") closeTerminalModal();
  });

  // Terminal command input (bottom bar)
  const terminalInput = document.getElementById("terminal-input");
  const terminalSendBtn = document.getElementById("terminal-send-btn");

  if (terminalInput) {
    terminalInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        e.preventDefault();
        sendTerminalCommandFromInput();
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
    if (e.key === "Escape" && document.getElementById("terminal-modal").classList.contains("active")) {
      closeTerminalModal();
    }
  });
}

// ✅ Start application — no duplicate listeners below this line
setupEventListeners();
init();