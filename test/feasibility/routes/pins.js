// NOT RUNNABLE TODAY: the workspace sandbox exposes no hash function, so this
// route cannot compute the fixtures' derived values. Kept as the artefact the
// route would need; see docs/plans/2026-11-26-changeset-feasibility-phase6.md.
// The codemode route for changeset-pins: one program.
const rel = await tools.read({path: "releases.json"});
const d = JSON.parse(rel.lines.map(l => l.text).join("\n"));
const files = await tools.glob({pattern: "services/*/service.yaml"});
for (const path of files.paths) {
  const name = path.split("/")[1];
  if (!d.stages[d.rollout.stage].includes(name) || !d.services[name]) continue;
  const body = await tools.read({path});
  const text = body.lines.map(l => l.text).join("\n");
  const replicas = Number((text.match(/^replicas: (\\d+)$/m) || [])[1] || 0);
  if (replicas < 2) continue;
  const tag = d.services[name].version + "-" + await sha1(d.salt + ":" + name + ":" + replicas);
  await tools.stageEdit({path, edits: [
    {old: text.match(/^image: .*$/m)[0], new: "image: ghcr.io/acme/" + name + ":" + tag}]});
}
return files.paths.length;
