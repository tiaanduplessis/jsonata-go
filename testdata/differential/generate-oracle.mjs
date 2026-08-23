#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';

const referenceCommit = '6c7e95fdbf4405a1e741852a7cd8cd985b4305bb';
const args = new Map();
for (let index = 2; index < process.argv.length; index += 2) {
  args.set(process.argv[index], process.argv[index + 1]);
}
const checkout = args.get('--checkout');
const corpusPath = args.get('--corpus') ?? 'testdata/differential/cases.json';
const outputPath = args.get('--output') ?? 'testdata/differential/oracle.json';
if (!checkout) {
  fail('usage: node testdata/differential/generate-oracle.mjs --checkout /path/to/jsonata-js');
}
let sourceCommit;
try {
  sourceCommit = execFileSync('git', ['-C', checkout, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
} catch (error) {
  fail(`cannot read jsonata-js checkout commit: ${error.message}`);
}
if (sourceCommit !== referenceCommit) {
  fail(`jsonata-js checkout is ${sourceCommit}; want ${referenceCommit}`);
}

const require = createRequire(import.meta.url);
const jsonata = require(path.resolve(checkout, 'src', 'jsonata.js'));
const corpus = JSON.parse(fs.readFileSync(corpusPath, 'utf8'));
if (corpus.referenceCommit !== referenceCommit) {
  fail(`corpus reference commit ${corpus.referenceCommit} does not match ${referenceCommit}`);
}

const results = [];
for (const testCase of corpus.cases) {
  try {
    const expression = jsonata(testCase.expression);
    const result = testCase.hasInput
      ? await expression.evaluate(testCase.input)
      : await expression.evaluate();
    if (typeof result === 'undefined') {
      results.push({ id: testCase.id, kind: 'undefined' });
    } else {
      results.push({ id: testCase.id, kind: 'value', value: result });
    }
  } catch (error) {
    const structured = { code: error.code ?? 'UNKNOWN' };
    if (typeof error.token === 'string') {
      structured.token = error.token;
    }
    results.push({ id: testCase.id, kind: 'error', error: structured });
  }
}

const oracle = {
  schemaVersion: 1,
  referenceName: 'jsonata-js-v2.2.2',
  referenceCommit,
  cases: results,
};
fs.writeFileSync(outputPath, `${JSON.stringify(oracle, null, 2)}\n`);

function fail(message) {
  process.stderr.write(`${message}\n`);
  process.exit(1);
}
