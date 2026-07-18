const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const TOML = require("@iarna/toml");
const Ajv2020 = require("ajv/dist/2020");
const addFormats = require("ajv-formats");

const root = path.resolve(__dirname, "..", "..");
const taplo = path.join(root, "node_modules", "@taplo", "cli", "dist", "cli.js");

function readToml(relativePath) {
  return TOML.parse(fs.readFileSync(path.join(root, relativePath), "utf8"));
}

function readJson(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(root, relativePath), "utf8"));
}

function runTaploSyntaxCheck() {
  const result = spawnSync(process.execPath, [
    taplo,
    "lint",
    "--config",
    "taplo.toml",
    "taplo.toml",
    "tests/schema/valid/xp2p-client.toml",
    "tests/schema/valid/xp2p-server.toml",
  ], {
    cwd: root,
    encoding: "utf8",
    stdio: "pipe",
  });

  if (result.stdout) process.stdout.write(result.stdout);
  if (result.stderr) process.stderr.write(result.stderr);
  if (result.status !== 0) process.exit(result.status || 1);
}

function assertTaploRule(config, include, schemaPath) {
  const rules = Array.isArray(config.rule) ? config.rule : [];
  const found = rules.some((rule) => (
    Array.isArray(rule.include)
      && rule.include.includes(include)
      && rule.schema
      && rule.schema.path === schemaPath
  ));
  if (!found) {
    throw new Error(`taplo.toml is missing ${include} -> ${schemaPath}`);
  }
}

function validator(schemaPath) {
  const ajv = new Ajv2020({ allErrors: true, strict: false });
  addFormats(ajv);
  return ajv.compile(readJson(schemaPath));
}

function validateFixture(validate, tomlPath, shouldPass) {
  const data = readToml(tomlPath);
  const valid = validate(data);
  if (valid !== shouldPass) {
    const suffix = validate.errors
      ? `\n${JSON.stringify(validate.errors, null, 2)}`
      : "";
    throw new Error(`${tomlPath} validation result was ${valid}, expected ${shouldPass}${suffix}`);
  }
}

runTaploSyntaxCheck();

const taploConfig = readToml("taplo.toml");
assertTaploRule(taploConfig, "**/xp2p-client.toml", "schemas/xp2p-client.schema.json");
assertTaploRule(taploConfig, "**/xp2p-server.toml", "schemas/xp2p-server.schema.json");

const validateClient = validator("schemas/xp2p-client.schema.json");
const validateServer = validator("schemas/xp2p-server.schema.json");

validateFixture(validateClient, "tests/schema/valid/xp2p-client.toml", true);
validateFixture(validateServer, "tests/schema/valid/xp2p-server.toml", true);
validateFixture(validateClient, "tests/schema/invalid/client/xp2p-client.toml", false);
validateFixture(validateServer, "tests/schema/invalid/server/xp2p-server.toml", false);

console.log("taplo schemas ok");
