const voteApi = window.DndVoteApp;
const voteElements = {
  missing: document.querySelector("#vote-missing"),
  content: document.querySelector("#vote-content"),
  title: document.querySelector("#vote-page-title"),
  form: document.querySelector("#vote-form"),
  voterName: document.querySelector("#voter-name"),
  rankingList: document.querySelector("#ranking-list"),
  rankingTemplate: document.querySelector("#ranking-item-template"),
  resultsCta: document.querySelector("#results-cta"),
};

const votePollId = voteApi.getPollIdFromLocation();
let votePoll = null;
let draggedItem = null;

void initializeVotePage();

async function initializeVotePage() {
  if (!votePollId) {
    renderMissingVotePoll("This poll link is missing or invalid.");
    return;
  }

  try {
    votePoll = await voteApi.getPoll(votePollId);
  } catch {
    renderMissingVotePoll("This poll link is missing or the poll could not be found on the server.");
    return;
  }

  voteElements.content.classList.remove("hidden");
  voteElements.title.textContent = votePoll.title;
  voteElements.resultsCta.href = voteApi.buildPageLink("results.html", votePoll.id);
  renderRankingList();
  voteElements.form.addEventListener("submit", (event) => {
    void submitVote(event);
  });
}

function renderMissingVotePoll(message) {
  voteElements.missing.classList.remove("hidden");
  voteElements.missing.textContent = message;
}

function renderRankingList() {
  voteElements.rankingList.innerHTML = "";

  votePoll.dates.forEach((date) => {
    const fragment = voteElements.rankingTemplate.content.cloneNode(true);
    const item = fragment.querySelector(".ranking-item");
    const label = fragment.querySelector(".ranking-label");
    const unavailable = fragment.querySelector(".unavailable-toggle");

    item.dataset.dateId = date.id;
    label.textContent = voteApi.formatDate(date.value);
    unavailable.addEventListener("change", () => {
      item.classList.toggle("ranking-item-unavailable", unavailable.checked);
      refreshRankingPositions();
    });

    wireDragAndDrop(item);
    voteElements.rankingList.append(fragment);
  });

  refreshRankingPositions();
}

function wireDragAndDrop(item) {
  item.addEventListener("dragstart", () => {
    draggedItem = item;
    item.classList.add("dragging");
  });

  item.addEventListener("dragend", () => {
    draggedItem = null;
    item.classList.remove("dragging");
    refreshRankingPositions();
  });

  item.addEventListener("dragover", (event) => {
    event.preventDefault();
    if (!draggedItem || draggedItem === item) {
      return;
    }

    const rect = item.getBoundingClientRect();
    const shouldInsertBefore = event.clientY < rect.top + rect.height / 2;
    voteElements.rankingList.insertBefore(
      draggedItem,
      shouldInsertBefore ? item : item.nextSibling
    );
  });
}

function refreshRankingPositions() {
  let availableIndex = 0;

  [...voteElements.rankingList.children].forEach((item) => {
    const checkbox = item.querySelector(".unavailable-toggle");
    const position = item.querySelector(".ranking-position");
    if (checkbox.checked) {
      position.textContent = "Unavailable";
      return;
    }

    availableIndex += 1;
    position.textContent = `Preference ${availableIndex}`;
  });
}

async function submitVote(event) {
  event.preventDefault();

  const voterName = voteElements.voterName.value.trim();
  if (!voterName) {
    window.alert("Please enter your name before submitting.");
    return;
  }

  const rankings = [];
  const unavailableDateIds = [];

  [...voteElements.rankingList.children].forEach((item) => {
    const checkbox = item.querySelector(".unavailable-toggle");
    if (checkbox.checked) {
      unavailableDateIds.push(item.dataset.dateId);
      return;
    }

    rankings.push(item.dataset.dateId);
  });

  if (rankings.length === 0) {
    window.alert("Please leave at least one date available and ranked.");
    return;
  }

  try {
    await voteApi.saveVote(votePoll.id, {
      voterName,
      rankings,
      unavailableDateIds,
    });
    window.location.href = voteApi.buildPageLink("results.html", votePoll.id);
  } catch (error) {
    window.alert(error.message);
  }
}
