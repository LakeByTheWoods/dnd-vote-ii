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

    item.dataset.dateId = date.id;
    item.classList.add(getWeekdayClass(date.value));
    label.textContent = formatVoteDate(date.value);
    unavailable.addEventListener("change", () => {
      item.classList.toggle("ranking-item-unavailable", unavailable.checked);
    });

    wireDragAndDrop(item);
    voteElements.rankingList.append(fragment);
  });
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
  });

  item.addEventListener("dragleave", (event) => {
    if (!item.contains(event.relatedTarget)) {
      item.classList.remove("drop-target-before", "drop-target-after");
    }
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
    renderVoteStatus("No existing ballot found for this name yet. You’re creating a new one.");
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

  existingVote.unavailableDateIds.forEach((dateId) => {
    const item = itemsByDateId.get(dateId);
    if (!item) {
      return;
    }
    const checkbox = item.querySelector(".unavailable-toggle");
    checkbox.checked = true;
    item.classList.add("ranking-item-unavailable");
  });

  renderVoteStatus("Loaded your existing ballot. Drag items to update it, then submit again.");
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
