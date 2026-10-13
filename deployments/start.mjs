import { spawn } from "node:child_process";

const apiPort = "8081";
const apiOrigin = `http://127.0.0.1:${apiPort}`;
const children = new Set();
let stopping = false;

function stop(exitCode) {
  if (stopping) return;
  stopping = true;
  process.exitCode = exitCode;
  for (const child of children) child.kill("SIGTERM");
  setTimeout(() => {
    for (const child of children) child.kill("SIGKILL");
  }, 10_000).unref();
}

process.on("SIGTERM", () => stop(0));
process.on("SIGINT", () => stop(0));

async function migrate() {
  const child = spawn("/app/bin/migrate", { stdio: "inherit" });
  children.add(child);
  try {
    await new Promise((resolve, reject) => {
      child.once("error", reject);
      child.once("close", (code) =>
        code === 0 ? resolve() : reject(new Error(`Migration exited with code ${code}`)),
      );
    });
  } finally {
    children.delete(child);
  }
}

function start(name, command, args, env) {
  const child = spawn(command, args, { env, stdio: "inherit" });
  children.add(child);
  child.once("error", (error) => {
    console.error(`${name} failed to start: ${error.message}`);
    stop(1);
  });
  child.once("close", (code) => {
    children.delete(child);
    if (!stopping) {
      console.error(`${name} exited with code ${code}`);
      stop(1);
    }
  });
}

async function main() {
  if (process.env.PORT === apiPort) {
    console.error(`Public PORT cannot be ${apiPort}; that port is reserved for the API`);
    process.exitCode = 1;
    return;
  }
  try {
    await migrate();
  } catch (error) {
    if (!stopping) {
      console.error(error.message);
      process.exitCode = 1;
    }
    return;
  }
  if (stopping) return;

  start("api", "/app/bin/server", [], {
    ...process.env,
    PORT: apiPort,
    API_BIND_HOST: "127.0.0.1",
  });
  start("worker", "/app/bin/worker", [], process.env);
  start("web", "node", ["/app/web/server.js"], {
    ...process.env,
    API_INTERNAL_URL: apiOrigin,
    HOSTNAME: "0.0.0.0",
  });
}

await main();
