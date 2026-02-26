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
      <h2 class="card__name">${escapeHTML(computer.name)}</h2>
      <p class="card__ip">${escapeHTML(computer.ip)}</p>
    </div>
    <div class="card__status">
      <span class="card__badge">${badgeText}</span>
    </div>
    <div class="card__footer">
      <span class="card__time">${timeText}</span>
      <button class="btn-check" data-id="${computer.id}">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <circle cx="12" cy="12" r="10"/>
          <polyline points="12 6 12 12 16 14"/>
        </svg>
        Check
      </button>
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

  // Attach event listener to check button
  document
    .querySelector(`#card-${computer.id} .btn-check`)
    .addEventListener("click", () => handlePingOne(computer.id));
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
  const name = document.getElementById("form-name").value.trim();
  const ip = document.getElementById("form-ip").value.trim();

  // Validate input
  if (!id || !name || !ip) {
    showToast("All fields are required", "error");
    return;
  }

  const submitButton = document.getElementById("form-submit");
  submitButton.disabled = true;
  submitButton.textContent = "Adding...";

  try {
    const newComputer = await addComputer({ id, name, ip });
    computers.push(newComputer);
    renderCard(newComputer);
    document.getElementById("count-total").textContent = computers.length;

    // Reset form
    document.getElementById("form-id").value = "";
    document.getElementById("form-name").value = "";
    document.getElementById("form-ip").value = "";

    showToast(`${newComputer.name} added successfully!`, "success");
    closeModal();
  } catch (error) {
    showToast(`Error: ${error.message}`, "error");
  } finally {
    submitButton.disabled = false;
    submitButton.textContent = "Add Computer";
  }
}

// ============================================================================
// Modal Management
// ============================================================================

/**
 * Opens the add computer modal
 */
function openModal() {
  document.getElementById("modal").classList.add("modal--open");
}

/**
 * Closes the add computer modal
 */
function closeModal() {
  document.getElementById("modal").classList.remove("modal--open");
}

// ============================================================================
// Notifications
// ============================================================================

/**
 * Shows a toast notification
 * @param {string} message - Notification message
 * @param {string} type - Notification type: "success" or "error"
 */
function showToast(message, type = "success") {
  const toast = document.getElementById("toast");
  toast.textContent = message;
  toast.className = `toast toast--${type} toast--show`;

  setTimeout(() => {
    toast.classList.remove("toast--show");
  }, 3000);
}

// ============================================================================
// Utilities
// ============================================================================

/**
 * Escapes HTML special characters to prevent XSS attacks
 * @param {string} text - Text to escape
 * @returns {string} Escaped text
 */
function escapeHTML(text) {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

// ============================================================================
// Application Initialization
// ============================================================================

/**
 * Initializes the application by loading computers and setting up event listeners
 */
async function init() {
  try {
    // Load computers from backend
    computers = await loadComputers();
    
    // Render all computers
    computers.forEach((c) => renderCard(c));
    
    // Initialize summary
    document.getElementById("count-total").textContent = computers.length;
    document.getElementById("count-online").textContent = "0";
    document.getElementById("count-offline").textContent = "0";

    console.log(`Loaded ${computers.length} computers successfully`);
  } catch (error) {
    // Show error message if backend is unavailable
    const errorDiv = document.getElementById("error");
    errorDiv.style.display = "block";
    errorDiv.textContent = "⚠️ Cannot reach server. Is the Go server running?";
    console.error("Failed to initialize application:", error);
  }
}

/**
 * Sets up event listeners for buttons and modal
 */
function setupEventListeners() {
  document.getElementById("btn-ping-all").addEventListener("click", handlePingAll);
  document.getElementById("btn-add").addEventListener("click", openModal);
  document.getElementById("modal-close").addEventListener("click", closeModal);
  document.getElementById("modal").addEventListener("click", (e) => {
    if (e.target.id === "modal") closeModal();
  });
  document.getElementById("add-form").addEventListener("submit", handleAddComputer);
}

// Start application
setupEventListeners();
init();
