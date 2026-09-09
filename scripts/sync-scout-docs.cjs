// Publish a versioned, agent-readable snapshot alongside the website.
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '..');
const version = '1.9.0';
const destination = `web/public/docs/authoring/${version}`;
const files = [
  ['prompts/challenge-scout-v1.9.md', 'guide.md'],
  ['docs/scout-index.md', 'index.md'],
  ['docs/cli.md', 'docs/cli.md'],
  ['docs/openapi-contract.md', 'docs/openapi-contract.md'],
  ...['validation-v0.3.md', 'visualization-v2.md', 'visualization-v1.md', 'frontier-admission-v1.md'].map(name => [`docs/specs/${name}`, `docs/specs/${name}`]),
  ...fs.readdirSync(path.join(root, 'protocol/schemas')).filter(name => name.endsWith('.json')).map(name => [`protocol/schemas/${name}`, `protocol/schemas/${name}`]),
];
// Include local Markdown references so agents can follow the specification without
// landing on a missing page. Each source is public repository documentation.
for (let i = 0; i < files.length; i++) {
  const [source, target] = files[i];
  if (!source.endsWith('.md') || source === 'docs/scout-index.md') continue;
  const body = fs.readFileSync(path.join(root, source), 'utf8');
  for (const match of body.matchAll(/\]\(([^)#]+\.md)(?:#[^)]*)?\)/g)) {
    if (/^[a-z]+:/i.test(match[1])) continue;
    const nextSource = path.posix.normalize(path.posix.join(path.posix.dirname(source), match[1]));
    const nextTarget = path.posix.normalize(path.posix.join(path.posix.dirname(target), match[1]));
    if (!nextSource.startsWith('docs/') || nextTarget.startsWith('../')) throw new Error(`Unsafe documentation reference: ${match[1]}`);
    if (!files.some(([,existing]) => existing === nextTarget)) files.push([nextSource, nextTarget]);
  }
}
for (const [source, target] of files) {
  const file = path.join(root, destination, target);
  const content = fs.readFileSync(path.join(root, source));
  if (process.argv.includes('--check')) {
    if (!fs.existsSync(file) || !fs.readFileSync(file).equals(content)) throw new Error(`Stale scout documentation: ${target}`);
  } else {
    fs.mkdirSync(path.dirname(file), {recursive: true});
    fs.writeFileSync(file, content);
  }
}
console.log(`Scout ${version}: ${files.length} documentation files ${process.argv.includes('--check') ? 'checked' : 'published'}`);
