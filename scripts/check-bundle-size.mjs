#!/usr/bin/env node

import { readdir, stat } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

const args = parseArgs(process.argv.slice(2));

if (!args.dir) {
  fail("Missing required --dir option.");
}

const rootDir = path.resolve(process.cwd(), args.dir);
const maxJsBytes = kbToBytes(args.maxJsKb);
const maxEntryJsBytes = kbToBytes(args.maxEntryJsKb);
const entryPrefix = args.entry || "index";

const jsFiles = (await listFiles(rootDir)).filter((file) => file.endsWith(".js"));

if (jsFiles.length === 0) {
  fail(`No JavaScript bundles found under ${rootDir}. Run the production build first.`);
}

const entries = await Promise.all(
  jsFiles.map(async (file) => {
    const info = await stat(file);
    return {
      file,
      name: path.basename(file),
      size: info.size
    };
  })
);

entries.sort((a, b) => b.size - a.size);

const violations = [];

if (maxJsBytes > 0) {
  for (const entry of entries) {
    if (entry.size > maxJsBytes) {
      violations.push(`${entry.name} is ${formatSize(entry.size)}, over --max-js-kb ${args.maxJsKb} KB`);
    }
  }
}

if (maxEntryJsBytes > 0) {
  const entryBundles = entries.filter((entry) => entry.name === `${entryPrefix}.js` || entry.name.startsWith(`${entryPrefix}-`));
  if (entryBundles.length === 0) {
    violations.push(`No entry bundle matching "${entryPrefix}" found under ${rootDir}`);
  }
  for (const entry of entryBundles) {
    if (entry.size > maxEntryJsBytes) {
      violations.push(`${entry.name} is ${formatSize(entry.size)}, over --max-entry-js-kb ${args.maxEntryJsKb} KB`);
    }
  }
}

const largest = entries.slice(0, 8).map((entry) => `${entry.name} ${formatSize(entry.size)}`);
console.log(`Bundle size check: ${entries.length} JS files scanned in ${path.relative(process.cwd(), rootDir) || rootDir}`);
console.log(`Largest JS bundles: ${largest.join(", ")}`);

if (violations.length > 0) {
  fail(`Bundle size budget exceeded:\n- ${violations.join("\n- ")}`);
}

console.log("Bundle size check passed.");

async function listFiles(dir) {
  const dirents = await readdir(dir, { withFileTypes: true });
  const files = await Promise.all(
    dirents.map((dirent) => {
      const fullPath = path.join(dir, dirent.name);
      if (dirent.isDirectory()) {
        return listFiles(fullPath);
      }
      if (dirent.isFile()) {
        return [fullPath];
      }
      return [];
    })
  );
  return files.flat();
}

function parseArgs(rawArgs) {
  const parsed = {};
  for (let index = 0; index < rawArgs.length; index += 1) {
    const arg = rawArgs[index];
    if (!arg.startsWith("--")) {
      fail(`Unexpected argument: ${arg}`);
    }
    const key = toCamelCase(arg.slice(2));
    const value = rawArgs[index + 1];
    if (!value || value.startsWith("--")) {
      fail(`Missing value for ${arg}`);
    }
    parsed[key] = value;
    index += 1;
  }
  return parsed;
}

function toCamelCase(value) {
  return value.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
}

function kbToBytes(value) {
  if (value === undefined) {
    return 0;
  }
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) {
    fail(`Invalid size budget: ${value}`);
  }
  return number * 1024;
}

function formatSize(bytes) {
  return `${(bytes / 1024).toFixed(2)} KB`;
}

function fail(message) {
  console.error(message);
  process.exit(1);
}
