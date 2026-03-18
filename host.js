const hostApi = window.DndVoteApp;
const hostElements = {
  form: document.querySelector("#host-form"),
  title: document.querySelector("#poll-title"),
  dateInputs: document.querySelector("#date-inputs"),
  dateTemplate: document.querySelector("#date-input-template"),
  addDate: document.querySelector("#add-date"),
  loadSample: document.querySelector("#load-sample"),
  clearStorage: document.querySelector("#clear-storage"),
  shareEmpty: document.querySelector("#share-empty"),
  shareCard: document.querySelector("#share-card"),
  shareTitle: document.querySelector("#share-title"),
  shareDateCount: document.querySelector("#share-date-count"),
  voteLink: document.querySelector("#vote-link"),
  resultsLink: document.querySelector("#results-link"),
  openVoteLink: document.querySelector("#open-vote-link"),
  openResultsLink: document.querySelector("#open-results-link"),
  pollList: document.querySelector("#poll-list"),
};

let activePoll = null;

void initializeHostPage();

async function initializeHostPage() {
  hostElements.addDate.addEventListener("click", () => addDateInput());
  hostElements.loadSample.addEventListener("click", loadSampleDates);
  hostElements.clearStorage.addEventListener("click", () => {
    void clearAllPolls();
  });
  hostElements.form.addEventListener("submit", (event) => {
    void submitPoll(event);
  });
  document.querySelectorAll("[data-copy-target]").forEach((button) => {
    button.addEventListener("click", () => copyFieldValue(button.dataset.copyTarget));
  });

  addDateInput();
  addDateInput();
  await renderSavedPolls();
}

function addDateInput(value = "") {
  const fragment = hostElements.dateTemplate.content.cloneNode(true);
  const row = fragment.querySelector(".date-input-row");
  const input = fragment.querySelector(".date-input");
  const removeButton = fragment.querySelector(".remove-date");

  input.value = value;
  removeButton.addEventListener("click", () => {
    if (hostElements.dateInputs.children.length <= 1) {
      input.value = "";
      return;
    }
    row.remove();
  });

  hostElements.dateInputs.append(fragment);
}

function loadSampleDates() {
  hostElements.title.value = "Waterdeep Campaign Night";
  hostElements.dateInputs.innerHTML = "";
  ["2026-03-25", "2026-03-27", "2026-03-29"].forEach((value) => addDateInput(value));
}

async function clearAllPolls() {
  const confirmed = window.confirm("Clear every poll and vote from the shared database?");
  if (!confirmed) {
    return;
  }

  try {
    await hostApi.clearPolls();
  } catch (error) {
    window.alert(error.message);
    return;
  }

  activePoll = null;
  hostElements.form.reset();
  hostElements.dateInputs.innerHTML = "";
  addDateInput();
  addDateInput();
  renderShareCard();
  await renderSavedPolls();
}

async function submitPoll(event) {
  event.preventDefault();

  const title = hostElements.title.value.trim();
  const dateValues = [...hostElements.dateInputs.querySelectorAll(".date-input")]
    .map((input) => input.value)
    .filter(Boolean)
    .filter((value, index, allValues) => allValues.indexOf(value) === index)
    .sort();

  if (!title || dateValues.length < 2) {
    window.alert("Please add a title and at least two unique dates.");
    return;
  }

  try {
    activePoll = await hostApi.createPoll(title, dateValues);
    renderShareCard();
    await renderSavedPolls();
  } catch (error) {
    window.alert(error.message);
  }
}

function renderShareCard() {
  const hasPoll = Boolean(activePoll);
  hostElements.shareEmpty.classList.toggle("hidden", hasPoll);
  hostElements.shareCard.classList.toggle("hidden", !hasPoll);

  if (!activePoll) {
    return;
  }

  const voteLink = hostApi.buildPageLink("vote.html", activePoll.id);
  const resultsLink = hostApi.buildPageLink("results.html", activePoll.id);

  hostElements.shareTitle.textContent = activePoll.title;
  hostElements.shareDateCount.textContent = `${activePoll.dates.length} possible dates`;
  hostElements.voteLink.value = voteLink;
  hostElements.resultsLink.value = resultsLink;
  hostElements.openVoteLink.href = voteLink;
  hostElements.openResultsLink.href = resultsLink;
}

async function renderSavedPolls() {
  hostElements.pollList.innerHTML = "";

  let polls;
  try {
    polls = await hostApi.getAllPolls();
  } catch (error) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.textContent = error.message;
    hostElements.pollList.append(empty);
    return;
  }

  if (polls.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.textContent = "No polls saved on the server yet.";
    hostElements.pollList.append(empty);
    return;
  }

  polls.forEach((poll) => {
    const item = document.createElement("article");
    item.className = "saved-poll";
    const voteLink = hostApi.buildPageLink("vote.html", poll.id);
    const resultsLink = hostApi.buildPageLink("results.html", poll.id);

    item.innerHTML = `
      <div>
        <h3>${escapeHtml(poll.title)}</h3>
        <p>${poll.dateCount} dates | ${poll.voteCount} vote${poll.voteCount === 1 ? "" : "s"}</p>
      </div>
      <div class="link-actions">
        <a class="button-link" href="${voteLink}">Voting Page</a>
        <a class="button-link" href="${resultsLink}">Results</a>
      </div>
    `;

    hostElements.pollList.append(item);
  });
}

function copyFieldValue(fieldId) {
  const input = document.getElementById(fieldId);
  if (!input.value) {
    return;
  }

  input.select();
  input.setSelectionRange(0, input.value.length);

  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(input.value).catch(() => window.alert("Copy failed."));
    return;
  }

  try {
    document.execCommand("copy");
  } catch {
    window.alert("Copy failed.");
  }
}

function escapeHtml(value) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
