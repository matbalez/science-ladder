// Keep the standalone explorer's participation instructions identical to the web app.
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '..');
const ts = require(path.join(root, 'web/node_modules/typescript'));
const file = path.join(root, 'web/public/showcase/quiet-echoes/index.html');
const source = fs.readFileSync(path.join(root, 'web/lib/solver-prompt.ts'), 'utf8');
let js = ts.transpileModule(source, {compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.ESNext}}).outputText.replace(/^import .*;\n/gm,'').replace(/^export /gm,'');
for (const [file, name] of [['triangle-reference.ts','TRIANGLE_SOURCE'],['multiply-reference.ts','MULTIPLY_SOURCE']]) {
 const ref = fs.readFileSync(path.join(root,'web/lib',file),'utf8').match(new RegExp(`export const ${name} = "([a-f0-9]+)"`));
 if (!ref) throw Error('Missing pinned reference '+name);
 js = `const ${name} = "${ref[1]}";\n`+js;
}
const html = fs.readFileSync(file,'utf8');
const start = html.indexOf('// BEGIN GENERATED SOLVER PROMPT') >= 0 ? html.indexOf('// BEGIN GENERATED SOLVER PROMPT') : html.indexOf('const CLI_SOURCE =');
const end = html.indexOf('const pinnedChallenge=', start);
if(start<0 || end<0) throw Error('Missing explorer boundaries');
const updated = html.slice(0,start)+'// BEGIN GENERATED SOLVER PROMPT\n'+js+'// END GENERATED SOLVER PROMPT\n'+html.slice(end);
if(process.argv.includes('--check')) { if(updated!==html) throw Error('Run node scripts/sync-explorer-prompt.cjs'); }
else fs.writeFileSync(file,updated);
