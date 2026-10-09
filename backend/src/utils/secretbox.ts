import {
  createCipheriv,
  createDecipheriv,
  createHash,
  randomBytes,
} from "node:crypto";

const PREFIX = "enc:v1:";
const DEV_KEY = "portico-dev-config-key-change-me";

let appKey = DEV_KEY;

export function setKey(key: string): void {
  appKey = key === "" ? DEV_KEY : key;
}

export function usingDevKey(): boolean {
  return appKey === DEV_KEY;
}

type ConfigMap = Record<string, unknown>;

function currentKey(): Buffer {
  return createHash("sha256").update(appKey).digest();
}

function isSecret(key: string): boolean {
  switch (key.toLowerCase().replaceAll("-", "_")) {
    case "password":
    case "passwd":
    case "api_key":
    case "apikey":
    case "token":
    case "secret":
    case "dsn":
    case "uri":
      return true;
    default:
      return false;
  }
}

function parse(raw: unknown): ConfigMap {
  if (raw == null) return {};
  if (typeof raw === "object" && !Buffer.isBuffer(raw))
    return { ...(raw as ConfigMap) };
  const text = String(raw).trim();
  if (text === "" || text === "null") return {};
  const cfg = JSON.parse(text) as ConfigMap | null;
  return cfg ?? {};
}

function encrypt(plain: string): string {
  if (plain === "" || plain.startsWith(PREFIX)) return plain;
  const nonce = randomBytes(12);
  const cipher = createCipheriv("aes-256-gcm", currentKey(), nonce);
  const enc = Buffer.concat([cipher.update(plain, "utf8"), cipher.final()]);
  const sealed = Buffer.concat([nonce, enc, cipher.getAuthTag()]);
  return PREFIX + sealed.toString("base64url");
}

function decrypt(value: string): string {
  if (!value.startsWith(PREFIX)) return value;
  const raw = Buffer.from(value.slice(PREFIX.length), "base64url");
  if (raw.length < 12) throw new Error("sealed value is too short");
  const nonce = raw.subarray(0, 12);
  const tag = raw.subarray(raw.length - 16);
  const data = raw.subarray(12, raw.length - 16);
  const decipher = createDecipheriv("aes-256-gcm", currentKey(), nonce);
  decipher.setAuthTag(tag);
  return Buffer.concat([decipher.update(data), decipher.final()]).toString(
    "utf8",
  );
}

function apply(raw: unknown, fn: (value: string) => string): ConfigMap {
  const cfg = parse(raw);
  for (const key of Object.keys(cfg)) {
    const value = cfg[key];
    if (typeof value !== "string" || !isSecret(key)) continue;
    cfg[key] = fn(value);
  }
  return cfg;
}

export function seal(raw: unknown): ConfigMap {
  return apply(raw, encrypt);
}

export function open(raw: unknown): ConfigMap {
  return apply(raw, decrypt);
}

export function redact(raw: unknown): ConfigMap {
  return apply(raw, () => "");
}

export function needsSeal(raw: unknown): boolean {
  const cfg = parse(raw);
  for (const key of Object.keys(cfg)) {
    const value = cfg[key];
    if (
      typeof value === "string" &&
      isSecret(key) &&
      value !== "" &&
      !value.startsWith(PREFIX)
    ) {
      return true;
    }
  }
  return false;
}

export function merge(existing: unknown, incoming: unknown): ConfigMap {
  const oldCfg = parse(existing);
  const next = parse(incoming);
  for (const key of Object.keys(next)) {
    const value = next[key];
    if (
      isSecret(key) &&
      typeof value === "string" &&
      value === "" &&
      key in oldCfg
    ) {
      next[key] = oldCfg[key];
    }
  }
  for (const key of Object.keys(oldCfg)) {
    if (!(key in next) && isSecret(key)) next[key] = oldCfg[key];
  }
  return next;
}
