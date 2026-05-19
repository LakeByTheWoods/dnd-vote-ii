(function attachAppApi() {
  async function apiRequest(path, options = {}) {
    const response = await window.fetch(path, {
      headers: {
        "Content-Type": "application/json",
        ...(options.headers ?? {}),
      },
      ...options,
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
      body: JSON.stringify({ title, dates: dateValues }),
    });
    return {
      ...data.poll,
      managementToken: data.managementToken,
    };
  }

  async function getPoll(pollId) {
    const data = await apiRequest(`/api/polls/${encodeURIComponent(pollId)}`);
    return data.poll;
  }

  async function getAllPolls() {
    const data = await apiRequest("/api/polls");
    return data.polls;
  }

  async function clearPolls(adminSecret) {
    await apiRequest("/api/polls", {
      method: "DELETE",
      headers: { "X-DnD-Vote-Admin-Secret": adminSecret },
    });
  }

  async function saveVote(pollId, vote) {
    const data = await apiRequest(`/api/polls/${encodeURIComponent(pollId)}/votes`, {
      method: "POST",
      body: JSON.stringify(vote),
    });
    return data.poll;
  }

  async function deleteVote(pollId, voteId, managementToken) {
    const data = await apiRequest(
      `/api/polls/${encodeURIComponent(pollId)}/votes/${encodeURIComponent(voteId)}`,
      {
        method: "DELETE",
        headers: { "X-DnD-Vote-Management-Token": managementToken },
      }
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

  function getManagementTokenFromLocation() {
    return new URLSearchParams(window.location.search).get("manage");
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
    apiRequest,
    clearPolls,
    createPoll,
    formatDate,
    getAllPolls,
    getManagementTokenFromLocation,
    getPoll,
    getPollIdFromLocation,
    getResults,
    buildPageLink,
    deleteVote,
    saveVote,
  };
})();
