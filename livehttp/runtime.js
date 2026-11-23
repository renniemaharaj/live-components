// Only pass HTML rendered by your application to this function.
export function applyUpdates(updates) {
  for (const { id, html } of updates) {
    const target = document.getElementById(id);

    if (target) {
      target.innerHTML = html;
    }
  }
}

export async function update(url, options = {}) {
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");

  const response = await fetch(url, { ...options, headers });

  if (!response.ok) {
    throw new Error(`Component update failed: ${response.status}`);
  }

  const { updates } = await response.json();
  applyUpdates(updates);

  return updates;
}

export function connect(url) {
  const address = new URL(url, location.href);

  if (address.protocol === "http:") {
    address.protocol = "ws:";
  } else if (address.protocol === "https:") {
    address.protocol = "wss:";
  }

  const socket = new WebSocket(address);

  socket.addEventListener("message", (event) => {
    try {
      const message = JSON.parse(event.data);

      if (message.error) {
        throw new Error(message.error);
      }

      applyUpdates(message.updates);
    } catch (error) {
      socket.dispatchEvent(new CustomEvent("updateerror", { detail: error }));
    }
  });

  return socket;
}
