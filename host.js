const hostApi = window.DndVoteApp;
const MONTHS_PER_VIEW = 4;
const WEEKDAY_LABELS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const hostElements = {
  form: document.querySelector("#host-form"),
  title: document.querySelector("#poll-title"),
  calendarGrid: document.querySelector("#calendar-grid"),
  prevMonths: document.querySelector("#prev-months"),
  nextMonths: document.querySelector("#next-months"),
  selectedDates: document.querySelector("#selected-dates"),
  clearSelectedDates: document.querySelector("#clear-selected-dates"),
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
let selectedDates = new Set();
let visibleMonthStart = startOfMonth(new Date());

void initializeHostPage();

async function initializeHostPage() {
  hostElements.prevMonths.addEventListener("click", () => {
    visibleMonthStart = addMonths(visibleMonthStart, -MONTHS_PER_VIEW);
    renderCalendars();
  });
  hostElements.nextMonths.addEventListener("click", () => {
    visibleMonthStart = addMonths(visibleMonthStart, MONTHS_PER_VIEW);
    renderCalendars();
  });
  hostElements.clearSelectedDates.addEventListener("click", clearSelectedDates);
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

  renderCalendars();
  renderSelectedDates();
  await renderSavedPolls();
}

function renderCalendars() {
  hostElements.calendarGrid.innerHTML = "";

  for (let monthOffset = 0; monthOffset < MONTHS_PER_VIEW; monthOffset += 1) {
    const monthDate = addMonths(visibleMonthStart, monthOffset);
    const calendar = document.createElement("section");
    calendar.className = "calendar-month";

    const title = document.createElement("h3");
    title.textContent = new Intl.DateTimeFormat(undefined, {
      month: "long",
      year: "numeric",
    }).format(monthDate);
    calendar.append(title);

    const weekdays = document.createElement("div");
    weekdays.className = "calendar-weekdays";
    WEEKDAY_LABELS.forEach((label) => {
      const weekday = document.createElement("span");
      weekday.textContent = label;
      weekdays.append(weekday);
    });
    calendar.append(weekdays);

    const daysGrid = document.createElement("div");
    daysGrid.className = "calendar-days";

    const firstDayOfMonth = startOfMonth(monthDate);
    const leadingBlankDays = (firstDayOfMonth.getDay() + 6) % 7;
    for (let blank = 0; blank < leadingBlankDays; blank += 1) {
      const spacer = document.createElement("span");
      spacer.className = "calendar-spacer";
      daysGrid.append(spacer);
    }

    const daysInMonth = getDaysInMonth(monthDate);
    const todayKey = toDateKey(new Date());

    for (let day = 1; day <= daysInMonth; day += 1) {
      const cellDate = new Date(monthDate.getFullYear(), monthDate.getMonth(), day);
      const dateKey = toDateKey(cellDate);
      const button = document.createElement("button");
      button.type = "button";
      button.className = "calendar-day";
      button.textContent = String(day);
      button.dataset.dateValue = dateKey;

      const isPast = dateKey < todayKey;
      if (selectedDates.has(dateKey)) {
        button.classList.add("calendar-day-selected");
      }
      if (dateKey === todayKey) {
        button.classList.add("calendar-day-today");
      }
      if (isPast) {
        button.classList.add("calendar-day-past");
        button.disabled = true;
      } else {
        button.addEventListener("click", () => toggleDateSelection(dateKey));
      }

      daysGrid.append(button);
    }

    calendar.append(daysGrid);
    hostElements.calendarGrid.append(calendar);
  }
}

function toggleDateSelection(dateKey) {
  if (selectedDates.has(dateKey)) {
    selectedDates.delete(dateKey);
  } else {
    selectedDates.add(dateKey);
  }

  renderCalendars();
  renderSelectedDates();
}

function renderSelectedDates() {
  const orderedDates = [...selectedDates].sort();
  hostElements.selectedDates.innerHTML = "";
  hostElements.selectedDates.classList.toggle("empty-selection", orderedDates.length === 0);

  if (orderedDates.length === 0) {
    hostElements.selectedDates.textContent = "Choose at least two days from the calendars above.";
    return;
  }

  orderedDates.forEach((dateValue) => {
    const chip = document.createElement("button");
    chip.type = "button";
    chip.className = "selected-date-chip";
    chip.innerHTML = `
      <span>${escapeHtml(hostApi.formatDate(dateValue))}</span>
      <strong aria-hidden="true">x</strong>
    `;
    chip.addEventListener("click", () => toggleDateSelection(dateValue));
    hostElements.selectedDates.append(chip);
  });
}

function clearSelectedDates() {
  selectedDates = new Set();
  renderCalendars();
  renderSelectedDates();
}

function loadSampleDates() {
  hostElements.title.value = "Waterdeep Campaign Night";
  selectedDates = new Set(["2026-03-25", "2026-03-27", "2026-03-29"]);
  visibleMonthStart = startOfMonth(new Date("2026-03-01T00:00:00"));
  renderCalendars();
  renderSelectedDates();
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
  selectedDates = new Set();
  visibleMonthStart = startOfMonth(new Date());
  hostElements.form.reset();
  renderCalendars();
  renderSelectedDates();
  renderShareCard();
  await renderSavedPolls();
}

async function submitPoll(event) {
  event.preventDefault();

  const title = hostElements.title.value.trim();
  const dateValues = [...selectedDates].sort();

  if (!title || dateValues.length < 2) {
    window.alert("Please add a title and select at least two unique dates.");
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

function startOfMonth(date) {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addMonths(date, count) {
  return new Date(date.getFullYear(), date.getMonth() + count, 1);
}

function getDaysInMonth(date) {
  return new Date(date.getFullYear(), date.getMonth() + 1, 0).getDate();
}

function toDateKey(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function escapeHtml(value) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
