// Chat SSE consumer — the only JS in the project. Vanilla, no build,
// loads as a regular <script>. Talks to /api/chat via fetch + reader so
// the user sees tokens stream in. Form state and history live in the DOM
// itself (transcript bubbles); each submit POSTs the running history
// alongside the new message because /api/chat is stateless.

(function () {
  const form = document.getElementById("chat-form");
  if (!form) return;

  const transcript = document.getElementById("chat-transcript");
  const empty = document.getElementById("chat-empty");
  const input = document.getElementById("chat-input");
  const send = document.getElementById("chat-send");
  const stop = document.getElementById("chat-stop");
  const errorBox = document.getElementById("chat-error");

  let abortCtl = null;

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const message = input.value.trim();
    if (!message || abortCtl) return;
    errorBox.hidden = true;
    errorBox.textContent = "";

    if (empty) empty.remove();

    // Snapshot the prior transcript BEFORE adding the new turn — the
    // server treats `message` and `history` as disjoint, so the new
    // user turn must not appear in both.
    const history = collectHistory();
    appendBubble("user", message);
    input.value = "";
    input.focus();
    const asst = appendBubble("asst", "");

    abortCtl = new AbortController();
    setSending(true);

    try {
      await streamChat({ message, history }, (delta) => {
        asst.textContent += delta;
        transcript.scrollTop = transcript.scrollHeight;
      }, abortCtl.signal);
    } catch (err) {
      // User-initiated aborts surface as DOMException("AbortError"); they
      // are the success path of the Stop button, not a failure to show.
      if (err && err.name === "AbortError") {
        // Leave the partial assistant bubble; user can re-prompt.
      } else {
        errorBox.textContent = err && err.message ? err.message : String(err);
        errorBox.hidden = false;
      }
    } finally {
      abortCtl = null;
      setSending(false);
    }
  });

  stop.addEventListener("click", () => {
    if (abortCtl) abortCtl.abort();
  });

  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      form.requestSubmit();
    }
  });

  function setSending(busy) {
    send.disabled = busy;
    input.disabled = busy;
    stop.hidden = !busy;
  }

  function appendBubble(role, text) {
    const div = document.createElement("div");
    div.className = "bubble bubble--" + role;
    div.dataset.role = role === "user" ? "user" : "assistant";
    div.textContent = text;
    transcript.appendChild(div);
    transcript.scrollTop = transcript.scrollHeight;
    return div;
  }

  function collectHistory() {
    return Array.from(transcript.querySelectorAll(".bubble")).map((el) => ({
      role: el.dataset.role,
      content: el.textContent || "",
    }));
  }

  async function streamChat(payload, onDelta, signal) {
    const res = await fetch("/api/chat", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(payload),
      signal,
    });
    if (!res.ok || !res.body) {
      const body = await res.text().catch(() => "");
      throw new Error(body || "chat failed (" + res.status + ")");
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";
    while (true) {
      const { value, done } = await reader.read();
      if (done) return;
      buf += decoder.decode(value, { stream: true });
      let idx;
      while ((idx = buf.indexOf("\n\n")) !== -1) {
        const block = buf.slice(0, idx);
        buf = buf.slice(idx + 2);
        const ev = parseSSE(block);
        if (!ev) continue;
        if (ev.event === "delta" && ev.data && ev.data.text) onDelta(ev.data.text);
        else if (ev.event === "done") return;
        else if (ev.event === "error") throw new Error((ev.data && ev.data.message) || "chat error");
      }
    }
  }

  function parseSSE(block) {
    let event = "message";
    const dataLines = [];
    for (const line of block.split("\n")) {
      if (line.startsWith("event:")) event = line.slice(6).trim();
      else if (line.startsWith("data:")) dataLines.push(line.slice(5).replace(/^ /, ""));
    }
    if (!dataLines.length) return { event };
    try {
      return { event, data: JSON.parse(dataLines.join("\n")) };
    } catch {
      return { event, data: { raw: dataLines.join("\n") } };
    }
  }
})();
