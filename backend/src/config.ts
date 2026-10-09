import fs from "node:fs";
import path from "node:path";
import dotenv from "dotenv";

export type Config = {
  httpAddr: string;
  dbDriver: string;
  dbHost: string;
  dbPort: string;
  dbUser: string;
  dbPassword: string;
  dbName: string;
  dbSSLMode: string;
  openAIAPIKey: string;
  openAIBaseURL: string;
  openAIModel: string;
  envFile: string;
};

function env(key: string, fallback: string): string {
  const v = process.env[key];
  return v != null && v !== "" ? v : fallback;
}

export function loadConfig(): Config {
  const local = path.resolve(".env");
  let envFile = "";
  if (fs.existsSync(local)) {
    dotenv.config({ path: local });
    envFile = local;
  }
  return {
    httpAddr: env("HTTP_ADDR", ":8080"),
    dbDriver: env("DB_DRIVER", "postgres"),
    dbHost: env("DB_HOST", "localhost"),
    dbPort: env("DB_PORT", "5432"),
    dbUser: env("DB_USER", "postgres"),
    dbPassword: env("DB_PASSWORD", "postgres"),
    dbName: env("DB_NAME", "portico"),
    dbSSLMode: env("DB_SSLMODE", "disable"),
    openAIAPIKey: process.env.OPENAI_API_KEY ?? "",
    openAIBaseURL: process.env.OPENAI_BASE_URL ?? "",
    openAIModel: process.env.OPENAI_MODEL ?? "",
    envFile,
  };
}

export function listenHostPort(addr: string): { host: string; port: number } {
  const raw = addr.trim();
  if (raw.startsWith(":")) {
    return { host: "0.0.0.0", port: Number(raw.slice(1)) };
  }
  const idx = raw.lastIndexOf(":");
  if (idx > 0) {
    return { host: raw.slice(0, idx), port: Number(raw.slice(idx + 1)) };
  }
  return { host: "0.0.0.0", port: Number(raw) };
}
