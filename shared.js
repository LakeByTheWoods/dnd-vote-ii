(function attachAppApi() {
  const hostKeyName = "dnd-vote-host-key";

  function setHostKey(key) {
    if (key) window.sessionStorage.setItem(hostKeyName, key);
    else window.sessionStorage.removeItem(hostKeyName);
  }

  function hasHostKey() {
    return Boolean(window.sessionStorage.getItem(hostKeyName));
  }

  function hostHeaders() {
    const key = window.sessionStorage.getItem(hostKeyName);
    return key ? { Authorization: `Bearer ${key}` } : {};
  }

  function receiptKey(pollId, voterName) {
    return `dnd-vote-receipt:${pollId}:${voterName.trim().toLowerCase()}`;
  }

  function hasVoteReceipt(pollId, voterName) {
    return Boolean(window.localStorage.getItem(receiptKey(pollId, voterName)));
  }

  async function apiRequest(path, options = {}) {
    const response = await window.fetch(path, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...(options.headers ?? {}),
      },
    });

    const data = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(data.error || "Request failed.");
    }

    return data;
  }

  async function createPoll(title, dateValues) {
    const data = await apiRequest("/api/polls", {
      method: "POST",
      headers: hostHeaders(),
      body: JSON.stringify({ title, dates: dateValues }),
    });
    return data.poll;
  }

  async function getPoll(pollId) {
    const data = await apiRequest(`/api/polls/${encodeURIComponent(pollId)}`);
    return data.poll;
  }

  async function getAllPolls() {
    const data = await apiRequest("/api/polls", { headers: hostHeaders() });
    return data.polls;
  }

  async function clearPolls() {
    await apiRequest("/api/polls", { method: "DELETE", headers: hostHeaders() });
  }

  async function saveVote(pollId, vote) {
    const key = receiptKey(pollId, vote.voterName);
    const receipt = window.localStorage.getItem(key);
    const data = await apiRequest(`/api/polls/${encodeURIComponent(pollId)}/votes`, {
      method: "POST",
      headers: { ...hostHeaders(), ...(receipt ? { "X-Vote-Token": receipt } : {}) },
      body: JSON.stringify(vote),
    });
    window.localStorage.setItem(key, data.voteToken);
    return data.poll;
  }

  async function deleteVote(pollId, voteId) {
    const data = await apiRequest(
      `/api/polls/${encodeURIComponent(pollId)}/votes/${encodeURIComponent(voteId)}`,
      { method: "DELETE", headers: hostHeaders() }
    );
    return data.poll;
  }

  async function getResults(pollId) {
    return apiRequest(`/api/polls/${encodeURIComponent(pollId)}/results`);
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
    if (url.pathname === "/" || url.pathname === "") {
      url.pathname = `/${fileName}`;
    } else {
      url.pathname = url.pathname.replace(/[^/]+$/, fileName);
    }
    url.search = `?poll=${encodeURIComponent(pollId)}`;
    url.hash = "";
    return url.toString();
  }

  window.DndVoteApp = {
    setHostKey,
    hasHostKey,
    hasVoteReceipt,
    apiRequest,
    clearPolls,
    createPoll,
    formatDate,
    getAllPolls,
    getPoll,
    getPollIdFromLocation,
    getResults,
    buildPageLink,
    deleteVote,
    saveVote,
  };
})();
