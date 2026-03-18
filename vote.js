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
    label.textContent = formatVoteDate(date.value);
    unavailable.addEventListener("change", () => {
      item.classList.toggle("ranking-item-unavailable", unavailable.checked);
    });

    wireDragAndDrop(item);
    voteElements.rankingList.append(fragment);
  });
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

function formatVoteDate(value) {
  const date = new Date(`${value}T00:00:00`);
  return new Intl.DateTimeFormat(undefined, {
    weekday: "long",
    day: "numeric",
    month: "long",
  }).format(date);
}
