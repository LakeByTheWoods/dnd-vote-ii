const resultsApi = window.DndVoteApp;
const resultsElements = {
  missing: document.querySelector("#results-missing"),
  content: document.querySelector("#results-content"),
  title: document.querySelector("#results-title"),
  subtitle: document.querySelector("#results-subtitle"),
  winnerLabel: document.querySelector("#winner-label"),
  winnerScore: document.querySelector("#winner-score"),
  tableBody: document.querySelector("#results-table-body"),
  votesList: document.querySelector("#votes-list"),
};

const resultsPollId = resultsApi.getPollIdFromLocation();
const resultsPoll = resultsPollId ? resultsApi.getPoll(resultsPollId) : null;

initializeResultsPage();

function initializeResultsPage() {
  if (!resultsPoll) {
    resultsElements.missing.classList.remove("hidden");
    resultsElements.missing.textContent =
      "This poll link is missing or does not exist in local storage for this browser.";
    return;
  }

  resultsElements.content.classList.remove("hidden");
  renderResultsPage();
}

function renderResultsPage() {
  const talliedDates = resultsApi.tallyResults(resultsPoll);
  const winner = talliedDates.find((date) => !date.disqualified) ?? null;

  resultsElements.title.textContent = resultsPoll.title;
  resultsElements.subtitle.textContent = `${resultsPoll.votes.length} vote${
    resultsPoll.votes.length === 1 ? "" : "s"
  } submitted`;
  resultsElements.winnerLabel.textContent = winner ? winner.label : "No eligible date";
  resultsElements.winnerScore.textContent = winner
    ? `${winner.score.toFixed(3)} points`
    : "Every date has been disqualified.";

  resultsElements.tableBody.innerHTML = "";
  talliedDates.forEach((date) => {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHtml(date.label)}</td>
      <td><span class="status-pill ${date.disqualified ? "status-disqualified" : "status-active"}">${
        date.disqualified ? "Disqualified" : "Eligible"
      }</span></td>
      <td>${date.disqualified ? "0.000" : date.score.toFixed(3)}</td>
      <td>${escapeHtml(date.unavailableBy.join(", ") || "None")}</td>
    `;
    resultsElements.tableBody.append(row);
  });

  resultsElements.votesList.innerHTML = "";
  if (resultsPoll.votes.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.textContent = "No votes yet.";
    resultsElements.votesList.append(empty);
    return;
  }

  resultsPoll.votes.forEach((vote) => {
    const item = document.createElement("article");
    item.className = "vote-item";
    const rankedSummary = vote.rankings
      .map((ranking, index) => {
        const date = resultsPoll.dates.find((pollDate) => pollDate.id === ranking.dateId);
        return `${index + 1}. ${date ? resultsApi.formatDate(date.value) : ranking.dateId}`;
      })
      .join(" | ");
    const unavailableSummary = vote.unavailableDateIds
      .map((dateId) => {
        const date = resultsPoll.dates.find((pollDate) => pollDate.id === dateId);
        return date ? resultsApi.formatDate(date.value) : dateId;
      })
      .join(", ");

    item.innerHTML = `
      <strong>${escapeHtml(vote.voterName)}</strong>
      <p>Ranked: ${escapeHtml(rankedSummary || "None")}</p>
      <p>Unavailable: ${escapeHtml(unavailableSummary || "None")}</p>
    `;
    resultsElements.votesList.append(item);
  });
}

function escapeHtml(value) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
