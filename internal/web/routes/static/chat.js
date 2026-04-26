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

  // Max height in pixels matches CSS .chat__input max-height (12rem).
  const INPUT_MAX_PX = 192;
  // Auto-follow only when the user is near the bottom — otherwise we'd
  // yank them away while they're reading earlier turns.
  const FOLLOW_THRESHOLD_PX = 200;

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
    autosize();
    input.focus();
    const asst = appendBubble("asst", "");

    abortCtl = new AbortController();
    setSending(true);

    try {
      await streamChat({ message, history }, (delta) => {
        asst.textContent += delta;
        followLatest();
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

  input.addEventListener("input", autosize);
  input.addEventListener("keydown", (e) => {
    // Plain Enter sends; Shift+Enter inserts a newline. Mirrors ChatGPT.
    if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
      e.preventDefault();
      form.requestSubmit();
    }
  });

  function setSending(busy) {
    send.disabled = busy;
    input.disabled = busy;
    stop.hidden = !busy;
  }

  // autosize grows the textarea to fit its content up to a hard cap.
  // Reset to "auto" first so scrollHeight reflects the natural height
  // even when the user has just deleted lines.
  function autosize() {
    input.style.height = "auto";
    input.style.height = Math.min(input.scrollHeight, INPUT_MAX_PX) + "px";
  }

  // appendBubble builds daisyUI's chat structure:
  //   <div class="chat chat-end|chat-start" data-role="...">
  //     <div class="chat-bubble">text</div>
  //   </div>
  // Returns the inner .chat-bubble so the caller can append streamed
  // delta text directly into it.
  function appendBubble(role, text) {
    const isUser = role === "user";
    const wrapper = document.createElement("div");
    wrapper.className = "chat " + (isUser ? "chat-end" : "chat-start");
    wrapper.dataset.role = isUser ? "user" : "assistant";

    const bubble = document.createElement("div");
    bubble.className = "chat-bubble" + (isUser ? " chat-bubble-primary" : "");
    bubble.textContent = text;

    wrapper.appendChild(bubble);
    transcript.appendChild(wrapper);
    followLatest();
    return bubble;
  }

  // followLatest scrolls the page to the bottom only when the user is
  // already near it. Avoids the "I'm reading history, stop dragging me
  // back to the live tokens" anti-pattern.
  function followLatest() {
    const el = document.scrollingElement || document.documentElement;
    const distance = el.scrollHeight - el.scrollTop - el.clientHeight;
    if (distance < FOLLOW_THRESHOLD_PX) {
      el.scrollTop = el.scrollHeight;
    }
  }

  function collectHistory() {
    return Array.from(transcript.querySelectorAll(".chat[data-role]")).map((el) => ({
      role: el.dataset.role,
      content: (el.querySelector(".chat-bubble") || el).textContent || "",
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
      throw new Error(await readErrorMessage(res));
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

  // readErrorMessage extracts the user-facing string from a non-OK chat
  // response. The server returns the {"error": "..."} envelope so the
  // user sees the prose, not the raw JSON.
  async function readErrorMessage(res) {
    const body = await res.text().catch(() => "");
    if (!body) return "chat failed (" + res.status + ")";
    try {
      const parsed = JSON.parse(body);
      if (parsed && typeof parsed.error === "string") return parsed.error;
    } catch {
      // not JSON — fall through and surface the raw body
    }
    return body;
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
