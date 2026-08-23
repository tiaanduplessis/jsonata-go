import fs from "node:fs/promises";
import crypto from "node:crypto";
import jsonata from "jsonata";

const sourcePath = new URL("matrix.json", import.meta.url);
const outputPath = new URL("corpus.json", import.meta.url);
const generatorPath = new URL("generate-oracle.mjs", import.meta.url);
const packageLockPath = new URL("package-lock.json", import.meta.url);
const referenceRoot = new URL("../reference/jsonata-js-v2.2.2/", import.meta.url);
const sourceData = await fs.readFile(sourcePath);
const source = JSON.parse(sourceData);

source.reference.generation = {
  matrix_sha256: sha256(sourceData),
  generator_sha256: sha256(await fs.readFile(generatorPath)),
  package_lock_sha256: sha256(await fs.readFile(packageLockPath))
};

for (const sample of source.cases) {
	let input = sample.input;
	if (sample.source) {
		const datasetPath = new URL(sample.source.path, referenceRoot);
		if (datasetPath.protocol !== "file:" || !datasetPath.href.startsWith(referenceRoot.href)) {
			throw new Error(`unsafe benchmark dataset path: ${sample.source.path}`);
		}
		const datasetData = await fs.readFile(datasetPath);
		input = JSON.parse(datasetData);
		sample.source.sha256 = sha256(datasetData);
	}
  const expression = jsonata(sample.expression);
  if (sample.custom_function === "double") {
    expression.registerFunction("double", value => value * 2, "<n:n>");
  }
  try {
    const value = await expression.evaluate(input);
    sample.oracle = {value};
  } catch (error) {
    sample.oracle = {
      error: {
        code: error.code,
        message: error.message,
        token: error.token,
        value: error.value,
        position: error.position
      }
    };
  }
	sample.input = JSON.stringify(input);
}

const generated = `${JSON.stringify(source, null, 2)}\n`;
if (process.argv.includes("--check")) {
	const frozen = await fs.readFile(outputPath, "utf8");
	if (frozen !== generated) {
		throw new Error("benchmark oracle is stale; run npm run generate and review corpus.json");
	}
} else {
	await fs.writeFile(outputPath, generated);
}

function sha256(data) {
	return crypto.createHash("sha256").update(data).digest("hex");
}
