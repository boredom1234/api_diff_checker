// State - global so functions can access it
let testCases = [];
let columns = ["Test A", "Test B"]; // Default columns

document.addEventListener("DOMContentLoaded", () => {
  // Initialize with sample data (one row)
  addTestCase("Scenario 1", {});

  // Event Listeners
  document
    .getElementById("add-column-btn")
    .addEventListener("click", () => addColumn());
  document
    .getElementById("add-testcase-btn")
    .addEventListener("click", () => addTestCase());
  document.getElementById("run-btn").addEventListener("click", runCheck);

  // Event Delegation for remove buttons
  document.addEventListener("click", (e) => {
    // Handle test case row removal
    if (e.target.closest(".remove-testcase-btn")) {
      const row = e.target.closest("tr.testcase-row");
      if (row) {
        const idx = parseInt(row.dataset.index);
        testCases.splice(idx, 1);
        renderTestCasesTable();
      }
    }
    // Handle column removal
    if (e.target.closest(".remove-col-btn")) {
      const idx = parseInt(e.target.closest(".remove-col-btn").dataset.index);
      if (columns.length <= 2) {
        alert("At least 2 columns are required for comparison.");
        return;
      }

      // Remove data for this column from all test cases
      const colName = columns[idx];
      testCases.forEach((tc) => {
        delete tc.commands[colName];
      });

      columns.splice(idx, 1);
      renderTestCasesTable();
    }
  });

  // Listen for column name changes
  document.addEventListener("input", (e) => {
    if (e.target.classList.contains("column-header-input")) {
      const idx = parseInt(e.target.dataset.index);
      const oldName = columns[idx];
      const newName = e.target.value.trim();

      if (newName && newName !== oldName) {
        // Update all test cases to use new key
        testCases.forEach((tc) => {
          if (tc.commands[oldName]) {
            tc.commands[newName] = tc.commands[oldName];
            delete tc.commands[oldName];
          }
        });
        columns[idx] = newName;
      }
    }

    // Save test case command changes
    if (e.target.classList.contains("testcase-command")) {
      const row = e.target.closest("tr.testcase-row");
      const idx = parseInt(row.dataset.index);
      const colName = e.target.dataset.column;
      if (testCases[idx]) {
        testCases[idx].commands[colName] = e.target.value;
      }
    }
    // Save test case name changes
    if (e.target.classList.contains("testcase-name")) {
      const row = e.target.closest("tr.testcase-row");
      const idx = parseInt(row.dataset.index);
      if (testCases[idx]) {
        testCases[idx].name = e.target.value;
      }
    }
  });
});

function addColumn(name = "") {
  const newName = name || `Test ${String.fromCharCode(65 + columns.length)}`; // A, B, C...
  columns.push(newName);
  renderTestCasesTable();
}

function addTestCase(name = "", commands = {}) {
  const newTestCase = {
    name: name || `Scenario ${testCases.length + 1}`,
    commands: {},
  };

  // Initialize commands for all current columns
  columns.forEach((col) => {
    newTestCase.commands[col] = commands[col] || "";
  });

  testCases.push(newTestCase);
  renderTestCasesTable();
}

function renderTestCasesTable() {
  const container = document.getElementById("testcases-container");

  let html = `
    <table class="testcases-table">
      <thead>
        <tr>
          <th class="tc-name-col">Scenario Name</th>
          ${columns
            .map(
              (col, idx) =>
                `<th class="tc-cmd-col">
                    <div class="column-header">
                        <input type="text" class="column-header-input" data-index="${idx}" value="${escapeHtml(
                  col
                )}" placeholder="Column Name">
                        <button class="remove-col-btn" data-index="${idx}" title="Remove Column">×</button>
                    </div>
                </th>`
            )
            .join("")}
          <th class="tc-action-col"></th>
        </tr>
      </thead>
      <tbody>
  `;

  if (testCases.length === 0) {
    html += `
      <tr>
        <td colspan="${columns.length + 2}" class="empty-row">
          No rows yet. Click "Add Row" to start.
        </td>
      </tr>
    `;
  } else {
    testCases.forEach((tc, idx) => {
      html += `
        <tr class="testcase-row" data-index="${idx}">
          <td class="tc-name-cell">
            <input type="text" class="testcase-name" value="${escapeHtml(
              tc.name
            )}" placeholder="Scenario name" />
          </td>
          ${columns
            .map(
              (col) => `
            <td class="tc-cmd-cell">
              <textarea 
                class="testcase-command" 
                data-column="${escapeHtml(col)}"
                placeholder="curl https://api..."
                rows="3"
              >${escapeHtml(tc.commands[col] || "")}</textarea>
            </td>
          `
            )
            .join("")}
          <td class="tc-action-cell">
            <button class="remove-testcase-btn" title="Remove Row">
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </td>
        </tr>
      `;
    });
  }

  html += `
      </tbody>
    </table>
  `;

  container.innerHTML = html;
}

async function runCheck() {
  const runBtn = document.getElementById("run-btn");
  const resultsPanel = document.getElementById("results-panel");
  const resultsContainer = document.getElementById("results-container");
  const resultsSummary = document.getElementById("results-summary");

  // Validate inputs
  if (columns.length < 2) {
    alert("At least 2 columns are required to compare.");
    return;
  }

  // Construct dummy versions map to satisfy backend validation
  // Since users paste full curl commands, the BaseUrl is technically not needed
  // IF they don't use the {{BASE_URL}} placeholder.
  // We provide a dummy URL just in case.
  const versions = {};
  columns.forEach((col) => {
    versions[col] = "http://placeholder-required-by-backend.com";
  });

  // Gather test cases data (re-read from DOM to get latest)
  const test_cases = [];
  document.querySelectorAll(".testcase-row").forEach((row) => {
    const idx = parseInt(row.dataset.index);
    const name = row.querySelector(".testcase-name").value.trim();
    const commands = {};
    row.querySelectorAll(".testcase-command").forEach((textarea) => {
      const col = textarea.dataset.column;
      const cmd = textarea.value.trim();
      if (cmd) commands[col] = cmd;
    });

    // Only add if there's at least one command
    if (Object.keys(commands).length > 0) {
      test_cases.push({ name: name || `Scenario ${idx + 1}`, commands });
    }
  });

  if (test_cases.length === 0) {
    alert("Please provide at least one row with commands.");
    return;
  }

  const keysOnly = document.getElementById("keys-only-toggle").checked;
  const config = { versions, test_cases, keys_only: keysOnly };

  // Set loading state
  runBtn.classList.add("loading");
  runBtn.disabled = true;
  resultsPanel.classList.add("hidden");
  resultsContainer.innerHTML = "";
  resultsSummary.innerHTML = "";

  try {
    const response = await fetch("/api/run", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    });

    if (!response.ok) {
      const errText = await response.text();
      throw new Error(errText || "Server Error");
    }

    const data = await response.json();
    renderResults(data);
    resultsPanel.classList.remove("hidden");

    // Scroll to results
    resultsPanel.scrollIntoView({ behavior: "smooth", block: "start" });
  } catch (err) {
    resultsContainer.innerHTML = `<div class="error-message">❌ ${escapeHtml(
      err.message
    )}</div>`;
    resultsPanel.classList.remove("hidden");
  } finally {
    runBtn.classList.remove("loading");
    runBtn.disabled = false;
  }
}

function renderResults(data) {
  const container = document.getElementById("results-container");
  const summaryContainer = document.getElementById("results-summary");

  let matchCount = 0;
  let diffCount = 0;
  let errorCount = 0;

  data.command_results.forEach((res) => {
    const diffs = res.diffs || [];
    const hasError = diffs.some((d) => d.error);
    const hasDiffs = diffs.some(
      (d) => d.diff_result && d.diff_result.summary !== "No top-level changes"
    );

    if (hasError) errorCount++;
    else if (hasDiffs) diffCount++;
    else matchCount++;

    const card = document.createElement("div");
    card.className = "result-card";

    // Determine status
    let statusClass, statusText, statusIcon;
    if (hasError) {
      statusClass = "status-error";
      statusText = "Error";
      statusIcon = "⚠️";
    } else if (hasDiffs) {
      statusClass = "status-diff";
      statusText = "Differences Found";
      statusIcon = "≠";
    } else {
      statusClass = "status-match";
      statusText = "Match";
      statusIcon = "✓";
    }

    // Header
    const header = document.createElement("div");
    header.className = "result-card-header";
    const displayName = res.test_case_name || "Unknown Scenario";
    header.innerHTML = `
            <div class="command-preview">${escapeHtml(displayName)}</div>
            <div class="status-pill ${statusClass}">
                <span>${statusIcon}</span>
                <span>${statusText}</span>
            </div>
        `;

    // Show commands per version/column
    if (res.commands && Object.keys(res.commands).length > 0) {
      const cmdsDiv = document.createElement("div");
      cmdsDiv.className = "commands-list";
      Object.entries(res.commands).forEach(([col, cmd]) => {
        const cmdItem = document.createElement("div");
        cmdItem.className = "command-item";
        cmdItem.innerHTML = `
          <span class="version-tag">${escapeHtml(col)}</span>
          <code>${escapeHtml(truncateCommand(cmd))}</code>
        `;
        cmdsDiv.appendChild(cmdItem);
      });
      header.appendChild(cmdsDiv);
    }

    // Body
    const body = document.createElement("div");
    body.className = "result-card-body";

    diffs.forEach((diff) => {
      const block = document.createElement("div");
      block.className = "comparison-block";

      // Comparison header
      const compHeader = document.createElement("div");
      compHeader.className = "comparison-header";
      compHeader.innerHTML = `
                <span class="version-tag old">${escapeHtml(
                  diff.version_a
                )}</span>
                <span class="comparison-arrow">→</span>
                <span class="version-tag new">${escapeHtml(
                  diff.version_b
                )}</span>
            `;
      block.appendChild(compHeader);

      if (diff.error) {
        const errDiv = document.createElement("div");
        errDiv.className = "error-message";
        errDiv.textContent = diff.error;
        block.appendChild(errDiv);
      } else if (diff.diff_result) {
        // Changes summary chips
        if (
          diff.diff_result.summary &&
          diff.diff_result.summary !== "No top-level changes"
        ) {
          const changesDiv = document.createElement("div");
          changesDiv.className = "changes-summary";

          const changes = parseChanges(diff.diff_result.summary);
          changes.forEach((change) => {
            const chip = document.createElement("span");
            chip.className = `change-chip ${change.type}`;
            chip.innerHTML = `${getChangeIcon(change.type)} ${escapeHtml(
              change.field
            )}`;
            changesDiv.appendChild(chip);
          });
          block.appendChild(changesDiv);

          // Side-by-side split view
          if (diff.old_content && diff.new_content) {
            const splitView = document.createElement("div");
            splitView.className = "side-by-side";

            // Old panel
            const oldPanel = document.createElement("div");
            oldPanel.className = "diff-panel old";
            oldPanel.innerHTML = `
              <div class="diff-panel-header">
                <span class="dot"></span>
                <span>${escapeHtml(diff.version_a)}</span>
                <span class="panel-label">LEFT</span>
              </div>
              <div class="diff-content">${highlightJson(diff.old_content)}</div>
            `;

            // New panel
            const newPanel = document.createElement("div");
            newPanel.className = "diff-panel new";
            newPanel.innerHTML = `
              <div class="diff-panel-header">
                <span class="dot"></span>
                <span>${escapeHtml(diff.version_b)}</span>
                <span class="panel-label">RIGHT</span>
              </div>
              <div class="diff-content">${highlightJson(diff.new_content)}</div>
            `;

            splitView.appendChild(oldPanel);
            splitView.appendChild(newPanel);
            block.appendChild(splitView);
          }

          // Unified diff view (collapsible)
          if (diff.diff_result.text_diff) {
            const unifiedToggle = document.createElement("details");
            unifiedToggle.className = "unified-toggle";
            unifiedToggle.innerHTML = `
              <summary>View Unified Diff</summary>
              <div class="unified-diff">${formatUnifiedDiff(
                diff.diff_result.text_diff
              )}</div>
            `;
            block.appendChild(unifiedToggle);
          }
        } else {
          const successDiv = document.createElement("div");
          successDiv.style.cssText =
            "padding: 1rem; background: var(--accent-green-bg); border-radius: var(--radius-md); color: var(--accent-green); display: flex; align-items: center; gap: 0.5rem;";
          successDiv.innerHTML = `<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg> Responses are identical`;
          block.appendChild(successDiv);
        }
      }
      body.appendChild(block);
    });

    card.appendChild(header);
    card.appendChild(body);
    container.appendChild(card);
  });

  // Summary badges
  if (matchCount > 0) {
    summaryContainer.innerHTML += `<span class="summary-badge badge-match">✓ ${matchCount} Match${
      matchCount > 1 ? "es" : ""
    }</span>`;
  }
  if (diffCount > 0) {
    summaryContainer.innerHTML += `<span class="summary-badge badge-diff">≠ ${diffCount} Diff${
      diffCount > 1 ? "s" : ""
    }</span>`;
  }
  if (errorCount > 0) {
    summaryContainer.innerHTML += `<span class="summary-badge badge-error">⚠ ${errorCount} Error${
      errorCount > 1 ? "s" : ""
    }</span>`;
  }
}

function parseChanges(summary) {
  const changes = [];
  const parts = summary.split(", ");

  parts.forEach((part) => {
    const addMatch = part.match(/Field '(.+?)' added/);
    const removeMatch = part.match(/Field '(.+?)' removed/);
    const changeMatch = part.match(/Field '(.+?)' changed/);

    if (addMatch) changes.push({ field: addMatch[1], type: "added" });
    else if (removeMatch)
      changes.push({ field: removeMatch[1], type: "removed" });
    else if (changeMatch)
      changes.push({ field: changeMatch[1], type: "modified" });
  });

  return changes;
}

function getChangeIcon(type) {
  if (type === "added")
    return '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>';
  if (type === "removed")
    return '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><line x1="5" y1="12" x2="19" y2="12"/></svg>';
  return '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>';
}

function highlightJson(jsonStr) {
  if (!jsonStr) return "";
  const escaped = escapeHtml(jsonStr);
  // Basic JSON syntax highlighting
  return escaped
    .replace(/"([^"]+)":/g, '<span class="json-key">"$1"</span>:')
    .replace(/: "([^"]*)"/g, ': <span class="json-string">"$1"</span>')
    .replace(/: (\d+\.?\d*)/g, ': <span class="json-number">$1</span>')
    .replace(/: (true|false)/g, ': <span class="json-bool">$1</span>')
    .replace(/: (null)/g, ': <span class="json-null">$1</span>');
}

function formatUnifiedDiff(text) {
  if (!text) return "";
  return text
    .split("\n")
    .map((line) => {
      const escaped = escapeHtml(line);
      if (line.startsWith("+") && !line.startsWith("+++")) {
        return `<span class="diff-line add">${escaped}</span>`;
      }
      if (line.startsWith("-") && !line.startsWith("---")) {
        return `<span class="diff-line remove">${escaped}</span>`;
      }
      if (line.startsWith("@@")) {
        return `<span class="diff-line info">${escaped}</span>`;
      }
      return `<span class="diff-line">${escaped}</span>`;
    })
    .join("\n");
}

function truncateCommand(cmd) {
  if (!cmd) return "";
  const normalized = cmd.replace(/\s+/g, " ").trim();
  return normalized.length > 80
    ? normalized.substring(0, 80) + "..."
    : normalized;
}

function escapeHtml(text) {
  if (!text) return "";
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}
