// NOT RUNNABLE TODAY: the workspace sandbox exposes no hash function, so this
// route cannot compute the fixtures' derived values. Kept as the artefact the
// route would need; see docs/plans/2026-11-26-changeset-feasibility-phase6.md.
// The codemode route for changeset-registry: one program.
const data = JSON.parse((await tools.read({path: "plugins/data.json"})).lines.map(l => l.text).join("\n"));
const pages = await tools.glob({pattern: "capabilities/*.go"});
for (const path of pages.paths) {
  const name = path.split("/")[1].replace(/\.go$/, "");
  if (name === "spec") continue;
  const p = data.plugins.find(x => x.name === name);
  if (!p || !p.enabled) continue;
  const digest = await sha1(name + "|" + p.kind + "|" + p.endpoint);
  const body = (await tools.read({path})).lines.map(l => l.text).join("\n");
  await tools.stageEdit({path, edits: [
    {old: body.match(/^\tDigest:   ".*",$/m)[0], new: '\tDigest:   "' + digest.slice(0, 8) + '",'}]});
}
return pages.paths.length;
