const voteApi = window.DndVoteApp;
const voteElements = {
  missing: document.querySelector("#vote-missing"),
  content: document.querySelector("#vote-content"),
  title: document.querySelector("#vote-page-title"),
  form: document.querySelector("#vote-form"),
  voterName: document.querySelector("#voter-name"),
  voteStatus: document.querySelector("#vote-status"),
  rankingList: document.querySelector("#ranking-list"),
  rankingTemplate: document.querySelector("#ranking-item-template"),
  submitButton: document.querySelector("#submit-vote-button"),
  resultsCta: document.querySelector("#results-cta"),
};

const votePollId = voteApi.getPollIdFromLocation();
const voteParams = new URLSearchParams(window.location.search);
const initialVoterName = voteParams.get("voter")?.trim() ?? "";
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
  voteElements.voterName.addEventListener("change", handleVoterLookup);
  voteElements.voterName.addEventListener("blur", handleVoterLookup);
  voteElements.form.addEventListener("submit", (event) => {
    void submitVote(event);
  });

  if (initialVoterName) {
    voteElements.voterName.value = initialVoterName;
    applyExistingVote(initialVoterName);
  }
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
    const moveUp = fragment.querySelector(".ranking-move-up");
    const moveDown = fragment.querySelector(".ranking-move-down");

    item.dataset.dateId = date.id;
    item.classList.add(getWeekdayClass(date.value));
    label.textContent = formatVoteDate(date.value);
    item.querySelector(".ranking-controls").setAttribute(
      "aria-label",
      `Ranking controls for ${label.textContent}`
    );
    moveUp.setAttribute("aria-label", `Move ${label.textContent} up`);
    moveDown.setAttribute("aria-label", `Move ${label.textContent} down`);
    moveUp.addEventListener("click", () => {
      moveRankingItem(item, -1);
    });
    moveDown.addEventListener("click", () => {
      moveRankingItem(item, 1);
    });
    unavailable.addEventListener("change", () => {
      item.classList.toggle("ranking-item-unavailable", unavailable.checked);
    });

    wireDragAndDrop(item);
    voteElements.rankingList.append(fragment);
  });

  updateRankingControls();
}

function handleVoterLookup() {
  const voterName = voteElements.voterName.value.trim();
  if (!voterName) {
    clearVoteStatus();
    clearBallotSelection();
    return;
  }

  applyExistingVote(voterName);
}

function wireDragAndDrop(item) {
  item.addEventListener("dragstart", () => {
    draggedItem = item;
    item.classList.add("dragging");
    voteElements.rankingList.classList.add("ranking-list-sorting");
  });

  item.addEventListener("dragend", () => {
    draggedItem = null;
    item.classList.remove("dragging");
    clearDropTargets();
    voteElements.rankingList.classList.remove("ranking-list-sorting");
  });

  item.addEventListener("dragover", (event) => {
    event.preventDefault();
    if (!draggedItem || draggedItem === item) {
      return;
    }

    const rect = item.getBoundingClientRect();
    const shouldInsertBefore = event.clientY < rect.top + rect.height / 2;
    clearDropTargets();
    item.classList.add(shouldInsertBefore ? "drop-target-before" : "drop-target-after");
    animateReorder(() => {
      voteElements.rankingList.insertBefore(
        draggedItem,
        shouldInsertBefore ? item : item.nextSibling
      );
    });
    updateRankingControls();
  });

  item.addEventListener("dragleave", (event) => {
    if (!item.contains(event.relatedTarget)) {
      item.classList.remove("drop-target-before", "drop-target-after");
    }
  });
}

function moveRankingItem(item, direction) {
  const sibling = direction < 0 ? item.previousElementSibling : item.nextElementSibling;
  if (!sibling) {
    return;
  }

  animateReorder(() => {
    if (direction < 0) {
      voteElements.rankingList.insertBefore(item, sibling);
      return;
    }

    voteElements.rankingList.insertBefore(sibling, item);
  });
  updateRankingControls();
}

function updateRankingControls() {
  const items = [...voteElements.rankingList.children];
  items.forEach((item, index) => {
    const moveUp = item.querySelector(".ranking-move-up");
    const moveDown = item.querySelector(".ranking-move-down");
    moveUp.disabled = index === 0;
    moveDown.disabled = index === items.length - 1;
  });
}

function animateReorder(mutateDom) {
  const items = [...voteElements.rankingList.children];
  const firstRects = new Map(items.map((entry) => [entry, entry.getBoundingClientRect()]));

  mutateDom();

  [...voteElements.rankingList.children].forEach((entry) => {
    const firstRect = firstRects.get(entry);
    if (!firstRect) {
      return;
    }

    const lastRect = entry.getBoundingClientRect();
    const deltaY = firstRect.top - lastRect.top;
    if (deltaY === 0) {
      return;
    }

    entry.animate(
      [
        { transform: `translateY(${deltaY}px)` },
        { transform: "translateY(0)" },
      ],
      {
        duration: 220,
        easing: "cubic-bezier(0.22, 1, 0.36, 1)",
      }
    );
  });
}

function clearDropTargets() {
  [...voteElements.rankingList.children].forEach((entry) => {
    entry.classList.remove("drop-target-before", "drop-target-after");
  });
}

function applyExistingVote(voterName) {
  const existingVote = findVoteByName(voterName);
  clearBallotSelection();

  if (!existingVote) {
    voteElements.submitButton.textContent = "Submit Vote";
    renderVoteStatus("No existing ballot found for this name yet. You are creating a new ballot.");
    return;
  }

  const itemsByDateId = new Map(
    [...voteElements.rankingList.children].map((item) => [item.dataset.dateId, item])
  );

  existingVote.rankings
    .slice()
    .sort((left, right) => left.position - right.position)
    .forEach((ranking) => {
      const item = itemsByDateId.get(ranking.dateId);
      if (item) {
        voteElements.rankingList.append(item);
      }
    });
  updateRankingControls();

  existingVote.unavailableDateIds.forEach((dateId) => {
    const item = itemsByDateId.get(dateId);
    if (!item) {
      return;
    }
    const checkbox = item.querySelector(".unavailable-toggle");
    checkbox.checked = true;
    item.classList.add("ranking-item-unavailable");
  });

  voteElements.submitButton.textContent = "Update Ballot";
  renderVoteStatus(`Editing existing ballot for ${existingVote.voterName}. Submitting will replace it.`);
}

function findVoteByName(voterName) {
  const normalizedName = voterName.trim().toLowerCase();
  return (
    votePoll.votes.find((vote) => vote.voterName.trim().toLowerCase() === normalizedName) ?? null
  );
}

function clearBallotSelection() {
  [...voteElements.rankingList.children].forEach((item) => {
    const checkbox = item.querySelector(".unavailable-toggle");
    checkbox.checked = false;
    item.classList.remove("ranking-item-unavailable");
  });

  votePoll.dates.forEach((date) => {
    const item = [...voteElements.rankingList.children].find((entry) => entry.dataset.dateId === date.id);
    if (item) {
      voteElements.rankingList.append(item);
    }
  });
  updateRankingControls();

  voteElements.submitButton.textContent = "Submit Vote";
}

function renderVoteStatus(message) {
  voteElements.voteStatus.textContent = message;
  voteElements.voteStatus.classList.remove("hidden");
}

function clearVoteStatus() {
  voteElements.voteStatus.textContent = "";
  voteElements.voteStatus.classList.add("hidden");
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
    const redirectUrl = new URL(voteApi.buildPageLink("results.html", votePoll.id));
    redirectUrl.searchParams.set("voter", voterName);
    window.location.href = redirectUrl.toString();
  } catch (error) {
    window.alert(error.message);
  }
}

function formatVoteDate(value) {
  const date = new Date(`${value}T00:00:00`);
  const weekday = new Intl.DateTimeFormat(undefined, { weekday: "long" }).format(date);
  const month = new Intl.DateTimeFormat(undefined, { month: "long" }).format(date);
  const day = date.getDate();
  return `${weekday} ${day}${getOrdinalSuffix(day)} ${month}`;
}

function getWeekdayClass(value) {
  const weekday = new Date(`${value}T00:00:00`).getDay();
  const classNames = [
    "ranking-sunday",
    "ranking-monday",
    "ranking-tuesday",
    "ranking-wednesday",
    "ranking-thursday",
    "ranking-friday",
    "ranking-saturday",
  ];
  return classNames[weekday];
}

function getOrdinalSuffix(day) {
  const remainder10 = day % 10;
  const remainder100 = day % 100;
  if (remainder10 === 1 && remainder100 !== 11) {
    return "st";
  }
  if (remainder10 === 2 && remainder100 !== 12) {
    return "nd";
  }
  if (remainder10 === 3 && remainder100 !== 13) {
    return "rd";
  }
  return "th";
}
