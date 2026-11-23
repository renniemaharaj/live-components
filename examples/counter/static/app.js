import { connect, update } from "/live.js";

const form = document.getElementById("counter-form");
const feedback = document.getElementById("counter-feedback");
let requests = Promise.resolve();
let connection = null;

if (new URLSearchParams(location.search).get("transport") !== "http") {
  const count = document.getElementById("current-count").value;
  const socket = connect(`/counter/live?count=${encodeURIComponent(count)}`);

  connection = new Promise((resolve, reject) => {
    socket.addEventListener("open", () => resolve(socket), { once: true });
    socket.addEventListener("error", () => {
      reject(new Error("Could not connect. Reload or choose HTTP updates."));
    }, { once: true });
    socket.addEventListener("close", () => {
      reject(new Error("Connection closed. Reload or choose HTTP updates."));
      feedback.textContent = "Connection closed. Reload or choose HTTP updates.";
    }, { once: true });
  });

  connection.catch((error) => {
    feedback.textContent = error.message;
  });
  socket.addEventListener("updateerror", (event) => {
    feedback.textContent = event.detail.message;
  });
}

form.addEventListener("submit", (event) => {
  event.preventDefault();
  const submitter = event.submitter;

  requests = requests.then(async () => {
    if (connection) {
      const socket = await connection;

      if (socket.readyState !== WebSocket.OPEN) {
        throw new Error("Connection closed. Reload or choose HTTP updates.");
      }

      socket.send(JSON.stringify({ action: submitter?.value || "" }));
      return;
    }

    // Read the latest hidden count after the preceding HTTP update finishes.
    await update(form.getAttribute("action"), {
      method: form.method,
      body: new URLSearchParams(new FormData(form, submitter)),
    });
  }).catch((error) => {
    feedback.textContent = error.message;
  });
});
