(function attachAppApi() {
  const STORAGE_KEY = "dnd-date-vote-polls";
  const SCORE_RATIO = 2 / 3;

  function loadPolls() {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return {};
    }

    try {
      const parsed = JSON.parse(raw);
      return parsed && typeof parsed === "object" ? parsed : {};
    } catch {
      localStorage.removeItem(STORAGE_KEY);
      return {};
    }
  }

  function savePolls(polls) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(polls));
  }

  function createPoll(title, dateValues) {
    const id = crypto.randomUUID();
    const poll = {
      id,
      title,
      createdAt: new Date().toISOString(),
      dates: dateValues.map((value) => ({
        id: crypto.randomUUID(),
        value,
      })),
      votes: [],
    };

    const polls = loadPolls();
    polls[id] = poll;
    savePolls(polls);
    return poll;
  }

  function getPoll(pollId) {
    const polls = loadPolls();
    return polls[pollId] ?? null;
  }

  function getAllPolls() {
    return Object.values(loadPolls()).sort((left, right) =>
      right.createdAt.localeCompare(left.createdAt)
    );
  }

  function clearPolls() {
    localStorage.removeItem(STORAGE_KEY);
  }

  function saveVote(pollId, vote) {
    const polls = loadPolls();
    const poll = polls[pollId];
    if (!poll) {
      return null;
    }

    const normalizedName = vote.voterName.trim().toLowerCase();
    const index = poll.votes.findIndex(
      (existingVote) => existingVote.voterName.trim().toLowerCase() === normalizedName
    );

    if (index >= 0) {
      poll.votes.splice(index, 1, vote);
    } else {
      poll.votes.push(vote);
    }

    polls[pollId] = poll;
    savePolls(polls);
    return poll;
  }

  function tallyResults(poll) {
    const dates = poll.dates.map((date) => ({
      ...date,
      label: formatDate(date.value),
      score: 0,
      unavailableBy: [],
      disqualified: false,
    }));
    const byId = new Map(dates.map((date) => [date.id, date]));

    poll.votes.forEach((vote) => {
      vote.unavailableDateIds.forEach((dateId) => {
        const result = byId.get(dateId);
        if (!result) {
          return;
        }

        result.disqualified = true;
        if (!result.unavailableBy.includes(vote.voterName)) {
          result.unavailableBy.push(vote.voterName);
        }
      });
    });

    poll.votes.forEach((vote) => {
      vote.rankings.forEach((ranking, index) => {
        const result = byId.get(ranking.dateId);
        if (!result || result.disqualified) {
          return;
        }

        result.score += SCORE_RATIO ** index;
      });
    });

    return dates.sort((left, right) => {
      if (left.disqualified !== right.disqualified) {
        return left.disqualified ? 1 : -1;
      }
      if (right.score !== left.score) {
        return right.score - left.score;
      }
      return left.value.localeCompare(right.value);
    });
  }

  function formatDate(value) {
    const date = new Date(`${value}T00:00:00`);
    return new Intl.DateTimeFormat(undefined, {
      weekday: "long",
      day: "numeric",
      month: "long",
      year: "numeric",
    }).format(date);
  }

  function getPollIdFromLocation() {
    return new URLSearchParams(window.location.search).get("poll");
  }

  function buildPageLink(fileName, pollId) {
    const url = new URL(window.location.href);
    url.pathname = url.pathname.replace(/[^/]+$/, fileName);
    url.search = `?poll=${encodeURIComponent(pollId)}`;
    url.hash = "";
    return url.toString();
  }

  window.DndVoteApp = {
    clearPolls,
    createPoll,
    formatDate,
    getAllPolls,
    getPoll,
    getPollIdFromLocation,
    buildPageLink,
    saveVote,
    tallyResults,
  };
})();
