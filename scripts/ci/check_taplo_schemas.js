const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const TOML = require("@iarna/toml");
const AjvDraft4 = require("ajv-draft-04");
const addFormats = require("ajv-formats");

const root = path.resolve(__dirname, "..", "..");
const taplo = path.join(root, "node_modules", "@taplo", "cli", "dist", "cli.js");

function readToml(relativePath) {
  return TOML.parse(fs.readFileSync(path.join(root, relativePath), "utf8"));
}

function readJson(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(root, relativePath), "utf8"));
}

function runTaploSyntaxCheck(files) {
  const result = spawnSync(process.execPath, [
    taplo,
    "lint",
    "--config",
    "taplo.toml",
    "taplo.toml",
    ...files,
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
  const schema = readJson(schemaPath);
  assertDraft4SchemaId(schema, schemaPath);
  const ajv = new AjvDraft4({ allErrors: true, strict: false });
  addFormats(ajv);
  return ajv.compile(schema);
}

function assertDraft4SchemaId(schema, schemaPath) {
  if (schema.$schema !== "http://json-schema.org/draft-04/schema#") {
    throw new Error(`${schemaPath} must use JSON Schema Draft 4 for Taplo compatibility`);
  }
  try {
    const id = new URL(schema.id || "");
    if (!id.protocol) throw new Error("missing protocol");
  } catch (err) {
    throw new Error(`${schemaPath} must use an absolute Draft 4 id URI`);
  }
  if (schema.$id) {
    throw new Error(`${schemaPath} must not use $id; Taplo supports Draft 4 id`);
  }
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

function fixtureFiles(relativeDir) {
  const absoluteDir = path.join(root, relativeDir);
  return fs.readdirSync(absoluteDir, { withFileTypes: true })
    .flatMap((entry) => {
      const relativePath = path.join(relativeDir, entry.name).replaceAll("\\", "/");
      return entry.isDirectory() ? fixtureFiles(relativePath) : [relativePath];
    })
    .filter((file) => file.endsWith(".toml"))
    .sort();
}

function main() {
  const taploConfig = readToml("taplo.toml");
  assertTaploRule(taploConfig, "**/xp2p-client.toml", "schemas/xp2p-client.schema.json");
  assertTaploRule(taploConfig, "**/xp2p-server.toml", "schemas/xp2p-server.schema.json");

  const validateClient = validator("schemas/xp2p-client.schema.json");
  const validateServer = validator("schemas/xp2p-server.schema.json");
  const requestedFiles = process.argv.slice(2);

  if (requestedFiles.length > 0) {
    runTaploSyntaxCheck(requestedFiles);
    for (const file of requestedFiles) {
      const name = path.basename(file);
      if (name === "xp2p-client.toml") {
        validateFixture(validateClient, file, true);
      } else if (name === "xp2p-server.toml") {
        validateFixture(validateServer, file, true);
      } else {
        throw new Error(`${file} does not match an xp2p TOML schema rule`);
      }
    }
  } else {
    const validFiles = [
      "tests/schema/valid/xp2p-client.toml",
      "tests/schema/valid/xp2p-server.toml",
    ];
    runTaploSyntaxCheck(validFiles);
    validateFixture(validateClient, validFiles[0], true);
    validateFixture(validateServer, validFiles[1], true);
    for (const file of fixtureFiles("tests/schema/invalid/client")) {
      validateFixture(validateClient, file, false);
    }
    for (const file of fixtureFiles("tests/schema/invalid/server")) {
      validateFixture(validateServer, file, false);
    }
  }

  console.log("taplo schemas ok");
}

try {
  main();
} catch (err) {
  console.error(err.message || err);
  process.exit(1);
}
