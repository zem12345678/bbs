import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const source = path.join(frontendRoot, "runtime-config.production.js");
const destination = path.join(frontendRoot, "dist", "config.js");

fs.copyFileSync(source, destination);
