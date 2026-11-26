// The codemode route for changeset-pins: one program for the whole rotation.
const raw = await tools.read({ path: "releases.json" });
const d = JSON.parse(raw.lines.map(l => l.text).join("\n"));
const stage = d.stages[d.rollout.stage];
const files = await tools.glob({ pattern: "services/*/service.yaml" });
const done = [];
for (const path of files.paths) {
  const name = path.split("/")[1];
  if (!stage.includes(name) || !d.services[name]) continue;
  const text = (await tools.read({ path })).lines.map(l => l.text).join("\n");
  const replicas = Number((text.match(/^replicas: (\d+)$/m) || [])[1] || 0);
  if (replicas < 2) continue;
  const tag = d.services[name].version + "-" + sha256(d.salt + ":" + name + ":" + replicas).slice(0, 7);
  const image = text.match(/^image: .*$/m)[0];
  const mode = text.match(/^mode: .*$/m)[0];
  await tools.stageEdit({ path, edits: [
    { old: image, new: "image: ghcr.io/acme/" + name + ":" + tag },
    { old: mode, new: mode + "\npin:\n  tag: " + tag + "\n  weight: " + (replicas * 10) + "\n" },
  ] });
  done.push([name, tag]);
}
const manText = (await tools.read({ path: "deploy/manifest.yaml" })).lines.map(l => l.text).join("\n") + "\n";
let next = manText.slice(0, manText.indexOf("  - service: "));
const tagOf = {};
for (const [n, t] of done) tagOf[n] = t;
for (const name of Object.keys(d.services)) {
  const tag = tagOf[name] || d.services[name].version;
  next += "  - service: " + name + "\n    tag: " + tag + "\n    releaseId: " +
    sha256(name + ":" + tag).slice(0, 10) + "\n";
}
for (const name of d.legacy) {
  const body = (await tools.read({ path: "services/" + name + "/service.yaml" }))
    .lines.map(l => l.text).join("\n");
  const tag = (body.match(/^image: .*:(\S+)$/m) || [])[1];
  next += "  - service: " + name + "\n    tag: " + tag + "\n    releaseId: " +
    sha256(name + ":" + tag).slice(0, 10) + "\n";
}
await tools.stageEdit({ path: "deploy/manifest.yaml", edits: [{ old: manText, new: next }] });
return done.length;
