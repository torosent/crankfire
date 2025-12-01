#!/usr/bin/env node

/*
 * Validate all Mermaid diagrams under docs/ by:
 * - Scanning markdown files for ```mermaid code fences
 * - Extracting each diagram
 * - Running @mermaid-js/mermaid-cli via npx for syntax validation
 *
 * Usage (from repo root):
 *   node scripts/validate-mermaid.js
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const { spawnSync } = require('child_process');

const DOCS_ROOT = path.join(__dirname, '..', 'docs');

function findMarkdownFiles(dir) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...findMarkdownFiles(fullPath));
    } else if (entry.isFile() && fullPath.endsWith('.md')) {
      files.push(fullPath);
    }
  }

  return files;
}

function extractMermaidBlocks(filePath) {
  const content = fs.readFileSync(filePath, 'utf8');
  const lines = content.split(/\r?\n/);

  const blocks = [];
  let inBlock = false;
  let current = [];
  let startLine = 0;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trim();

    if (!inBlock && trimmed === '```mermaid') {
      inBlock = true;
      current = [];
      // Record 1-based line number of the first diagram line (after the fence)
      startLine = i + 2;
      continue;
    }

    if (inBlock && trimmed === '```') {
      blocks.push({
        content: current.join('\n'),
        startLine,
        endLine: i + 1,
      });
      inBlock = false;
      current = [];
      continue;
    }

    if (inBlock) {
      current.push(line);
    }
  }

  return blocks;
}

function validateBlock(content) {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'crankfire-mermaid-'));
  const inputPath = path.join(tmpDir, 'diagram.mmd');
  const outputPath = path.join(tmpDir, 'out.svg');

  fs.writeFileSync(inputPath, content, 'utf8');

  // Use npx so we don't need a permanent devDependency in the repo.
  const result = spawnSync(
    'npx',
    ['-q', '@mermaid-js/mermaid-cli@latest', '-i', inputPath, '-o', outputPath],
    { encoding: 'utf8' }
  );

  // Clean up temp files/directories.
  try {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  } catch (_) {
    // Ignore cleanup errors.
  }

  if (result.error) {
    return {
      ok: false,
      error: `Failed to run npx/@mermaid-js/mermaid-cli: ${result.error.message}`,
    };
  }

  if (result.status === 0) {
    return { ok: true };
  }

  const stderr = (result.stderr || '').trim();
  const stdout = (result.stdout || '').trim();

  return {
    ok: false,
    error: stderr || stdout || 'Unknown Mermaid CLI error',
  };
}

function main() {
  if (!fs.existsSync(DOCS_ROOT)) {
    console.error(`Docs directory not found at: ${DOCS_ROOT}`);
    process.exit(1);
  }

  const files = findMarkdownFiles(DOCS_ROOT);

  let totalBlocks = 0;
  const failures = [];

  for (const file of files) {
    const relPath = path.relative(process.cwd(), file);
    const blocks = extractMermaidBlocks(file);

    if (blocks.length === 0) {
      continue;
    }

    blocks.forEach((block, index) => {
      totalBlocks++;
      const label = `${relPath}: block ${index + 1} (approx line ${block.startLine})`;
      process.stdout.write(`Validating ${label} ... `);

      const result = validateBlock(block.content);

      if (result.ok) {
        console.log('OK');
      } else {
        console.log('FAIL');
        failures.push({
          file: relPath,
          index: index + 1,
          startLine: block.startLine,
          error: result.error,
        });
      }
    });
  }

  console.log(`\nChecked ${totalBlocks} Mermaid block(s) across ${files.length} markdown file(s).`);

  if (failures.length > 0) {
    console.log('\nFailures:');
    for (const f of failures) {
      console.log(`- ${f.file}:${f.startLine} (block ${f.index})`);
      const lines = f.error.split('\n').filter(Boolean).slice(0, 5);
      for (const line of lines) {
        console.log(`  ${line}`);
      }
    }
    process.exit(1);
  }

  console.log('All Mermaid diagrams validated successfully.');
}

main();
