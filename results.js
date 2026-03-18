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
let resultsPoll = null;

void initializeResultsPage();

async function initializeResultsPage() {
  if (!resultsPollId) {
    renderMissing("This poll link is missing or invalid.");
    return;
  }

  let resultData;
  try {
    resultData = await resultsApi.getResults(resultsPollId);
  } catch {
    renderMissing("This poll link is missing or the poll could not be found on the server.");
    return;
  }

  resultsPoll = resultData.poll;
  resultsElements.content.classList.remove("hidden");
  renderResultsPage(resultData.talliedDates, resultData.winner);
}

function renderMissing(message) {
  resultsElements.missing.classList.remove("hidden");
  resultsElements.missing.textContent = message;
}

function renderResultsPage(talliedDates, winner) {
  resultsElements.title.textContent = resultsPoll.title;
  resultsElements.subtitle.textContent = `${resultsPoll.votes.length} vote${
    resultsPoll.votes.length === 1 ? "" : "s"
  } submitted`;
  resultsElements.winnerLabel.textContent = winner ? resultsApi.formatDate(winner.value) : "No eligible date";
  resultsElements.winnerScore.textContent = winner
    ? `${winner.record} head-to-head record`
    : "Every date has been disqualified.";

  resultsElements.tableBody.innerHTML = "";
  talliedDates.forEach((date) => {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td>${escapeHtml(resultsApi.formatDate(date.value))}</td>
      <td><span class="status-pill ${date.disqualified ? "status-disqualified" : "status-active"}">${
        date.disqualified ? "Disqualified" : "Eligible"
      }</span></td>
      <td>${date.disqualified ? "-" : escapeHtml(date.record)}</td>
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
