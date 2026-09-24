import { appendFileSync } from "node:fs";
import { createServer } from "node:http";

const argumentsByName = new Map();
for (let index = 2; index < process.argv.length; index += 2) {
  argumentsByName.set(process.argv[index], process.argv[index + 1]);
}
const port = Number(argumentsByName.get("--port"));
const logPath = argumentsByName.get("--log");
if (!Number.isInteger(port) || port < 1 || port > 65535 || !logPath) {
  throw new Error("valid --port and --log arguments are required");
}

function respond(response, status, payload) {
  const encoded = JSON.stringify(payload);
  response.writeHead(status, {
    "content-type": "application/json",
    "content-length": Buffer.byteLength(encoded),
  });
  response.end(encoded);
}

createServer((request, response) => {
  if (request.method !== "POST") {
    respond(response, 405, { errors: [{ code: "method_not_allowed" }] });
    return;
  }
  let encoded = "";
  request.setEncoding("utf8");
  request.on("data", (chunk) => {
    encoded += chunk;
    if (encoded.length > 64 * 1024) request.destroy();
  });
  request.on("end", () => {
    try {
      const payload = JSON.parse(encoded);
      if (request.url?.endsWith("/push/send")) {
        const notificationId = payload.data.notificationId;
        if (payload.data.version !== "1" || typeof notificationId !== "string") {
          throw new Error("invalid push data");
        }
        appendFileSync(logPath, `${JSON.stringify(payload)}\n`, "utf8");
        respond(response, 200, {
          data: { status: "ok", id: `receipt-${notificationId}` },
        });
        return;
      }
      if (request.url?.endsWith("/push/getReceipts") && Array.isArray(payload.ids)) {
        respond(response, 200, {
          data: Object.fromEntries(payload.ids.map((id) => [id, { status: "ok" }])),
        });
        return;
      }
    } catch {
      // The acceptance provider exposes only a stable provider error.
    }
    respond(response, 400, { errors: [{ code: "malformed_request" }] });
  });
}).listen(port, "0.0.0.0");
