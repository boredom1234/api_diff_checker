document.addEventListener("DOMContentLoaded", () => {
  // Initialize with sample data
  addVersion("v1", "");
  addVersion("v2", "");
  addCommand("curl {{BASE_URL}}");

  // Event Listeners
  document
    .getElementById("add-version-btn")
    .addEventListener("click", () => addVersion());
  document
    .getElementById("add-command-btn")
    .addEventListener("click", () => addCommand());
  document.getElementById("run-btn").addEventListener("click", runCheck);

  // Event Delegation for remove buttons
  document.addEventListener("click", (e) => {
    if (e.target.closest(".remove-btn")) {
      const row = e.target.closest(".item-row");
      row.style.animation = "slideOut 0.2s ease-out forwards";
      setTimeout(() => row.remove(), 200);
    }
  });
});

function addVersion(name = "", url = "") {
  const container = document.getElementById("versions-container");
  const template = document.getElementById("version-item-template");
  const clone = template.content.cloneNode(true);

  if (name) clone.querySelector(".version-name").value = name;
  if (url) clone.querySelector(".version-url").value = url;

  container.appendChild(clone);
}

function addCommand(cmd = "") {
  const container = document.getElementById("commands-container");
  const template = document.getElementById("command-item-template");
  const clone = template.content.cloneNode(true);

  if (cmd) clone.querySelector(".command-input").value = cmd;

  container.appendChild(clone);
}

async function runCheck() {
  const runBtn = document.getElementById("run-btn");
  const resultsPanel = document.getElementById("results-panel");
  const resultsContainer = document.getElementById("results-container");
  const resultsSummary = document.getElementById("results-summary");

  // Gather data
  const versions = {};
  document.querySelectorAll(".version-row").forEach((item) => {
    const name = item.querySelector(".version-name").value.trim();
    const url = item.querySelector(".version-url").value.trim();
    if (name && url) versions[name] = url;
  });

  const commands = [];
  document.querySelectorAll(".command-row").forEach((item) => {
    const cmd = item.querySelector(".command-input").value.trim();
    if (cmd) commands.push(cmd);
  });

  if (Object.keys(versions).length < 2) {
    alert("Please provide at least 2 versions to compare.");
    return;
  }
  if (commands.length === 0) {
    alert("Please provide at least one command.");
    return;
  }

  const keysOnly = document.getElementById("keys-only-toggle").checked;
  const config = { versions, commands, keys_only: keysOnly };

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
    header.innerHTML = `
            <div class="command-preview">${escapeHtml(
              truncateCommand(res.command)
            )}</div>
            <div class="status-pill ${statusClass}">
                <span>${statusIcon}</span>
                <span>${statusText}</span>
            </div>
        `;

    // Body
    const body = document.createElement("div");
    body.className = "result-card-body";

    diffs.forEach((diff) => {
      const block = document.createElement("div");
      block.className = "comparison-block";

      // Version comparison header
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
                <span class="panel-label">OLD</span>
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
                <span class="panel-label">NEW</span>
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

    // Execution info
    if (res.execution_info && res.execution_info.length > 0) {
      const execInfo = document.createElement("div");
      execInfo.className = "exec-info";
      execInfo.innerHTML =
        '<div class="exec-info-title">Execution Details</div>';

      res.execution_info.forEach((info) => {
        const item = document.createElement("div");
        item.className = "exec-item";

        const statusClass = info.error ? "error" : "success";
        const statusIcon = info.error
          ? '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>'
          : '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>';

        const statusText = info.error ? `Failed: ${info.error}` : `Saved`;

        item.innerHTML = `
                    <span class="exec-version">[${escapeHtml(
                      info.version
                    )}]</span>
                    <span class="exec-status ${statusClass}">${statusIcon} ${escapeHtml(
          statusText
        )}</span>
                    ${
                      info.file
                        ? `<span class="exec-file">${escapeHtml(
                            info.file
                          )}</span>`
                        : ""
                    }
                `;
        execInfo.appendChild(item);
      });
      body.appendChild(execInfo);
    }

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

// Add slideOut animation
const style = document.createElement("style");
style.textContent = `
    @keyframes slideOut {
        from { opacity: 1; transform: translateX(0); }
        to { opacity: 0; transform: translateX(-20px); }
    }
`;
document.head.appendChild(style);
