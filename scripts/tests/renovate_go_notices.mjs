import fs from 'node:fs';
import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import os from 'node:os';
// Optional integration check using the real, pinned Renovate implementation.
// See docs/ci.md for installation and invocation; no GitHub API calls occur.
const renovateDir = process.env.RENOVATE_TEST_DIR;
assert(renovateDir, 'Set RENOVATE_TEST_DIR to a renovate package directory');
const load = (file) => import(pathToFileURL(path.join(renovateDir, 'dist', file)));
const { GlobalConfig } = await load('config/global.js');
const { extractPackageFile } = await load('modules/manager/custom/regex/index.js');
const { doAutoReplace } = await load('workers/repository/update/branch/auto-replace.js');
const { regexEngineStatus } = await load('util/regex.js');
const { applyPackageRules } = await load('util/package-rules/index.js');
const { compile } = await load('util/template/index.js');
const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'renovate-notices-'));
process.on('exit', () => fs.rmSync(temp, { recursive: true, force: true }));
GlobalConfig.set({localDir:temp});
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const config=JSON.parse(fs.readFileSync(root+'/renovate.json'));
const manager=config.customManagers[1];
const content=fs.readFileSync(root+'/THIRD_PARTY_NOTICES.md','utf8');
assert.equal(regexEngineStatus.type,'available');
let extracted=extractPackageFile(content,'THIRD_PARTY_NOTICES.md',manager);
assert.equal(extracted.deps.length,9);
assert(!extracted.deps.some(d=>d.depName.includes('testify')));
for(const [name,newValue] of [['github.com/ulikunitz/xz','v0.5.17'],['github.com/urfave/cli/v3','v3.14.0'],['github.com/diamondburned/gotk4/pkg','v0.4.2'],['golang.org/x/sys','v0.45.0']]) {
 const depIndex=extracted.deps.findIndex(d=>d.depName===name);
 const dep=extracted.deps[depIndex];
 const upgrade={...manager,...dep,manager:'regex',packageFile:'THIRD_PARTY_NOTICES.md',depIndex,newValue,autoReplaceGlobalMatch:true};
 const updated=await doAutoReplace(upgrade,content,false);
 assert(updated);
 assert.equal(updated.split('\n').filter((l,i)=>l!==content.split('\n')[i]).length,1);
 const row=updated.split('\n').find(l=>l.startsWith('| '+name+' '));
 assert(row.includes(name+' '+newValue+' |'));
 assert(row.includes('/tree/'+(name.includes('gotk4')?'pkg/':'')+newValue));
 if(name.includes('gotk4')) assert(row.endsWith('/pkg |'));
 const second=await doAutoReplace(upgrade,updated,false);
 assert.equal(second,updated);
 console.log(name+': extraction, exact tag replacement, one-row scope, idempotency PASS');
}
console.log('9 main requirements only; native, gotk4 prose and historical go.sum preserved; RE2 '+regexEngineStatus.type);

for (const dep of extracted.deps) {
 const groups=[];
 for (const [manager,packageFile] of [['gomod','go.mod'],['regex','THIRD_PARTY_NOTICES.md']]) {
  const applied=await applyPackageRules({...dep,manager,packageFile,packageRules:config.packageRules});
  assert(applied.groupSingleUpdates);
  assert(applied.autoReplaceGlobalMatch);
  groups.push(compile(applied.groupName,applied));
 }
 assert.equal(groups[0],groups[1]);
 assert.equal(groups[0],'Go module '+dep.depName);
}
const indirect=await applyPackageRules({manager:'gomod',packageFile:'go.mod',depName:'golang.org/x/sync',depType:'indirect',enabled:false,packageRules:config.packageRules});
assert.equal(indirect.enabled,true);
for (const packageFile of ['scripts/ci-staticcheck/go.mod','tools/layer-shell-probe/go.mod','.github/ci-tools.env']) {
 const applied=await applyPackageRules({manager:'gomod',packageFile,depName:'golang.org/x/sync',packageRules:config.packageRules});
 assert.equal(applied.groupName,undefined);
}
console.log('All 9 Go requirements and notices share per-module groups; indirect updates enabled; tool/probe modules isolated PASS');
