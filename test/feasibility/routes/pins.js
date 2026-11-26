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

const manLines = (await tools.read({ path: "deploy/manifest.yaml" })).lines.map(l => l.text);
const manText = manLines.join("\n");
const tagOf = {};
for (const [n, t] of done) tagOf[n] = t;

// Rewrite the manifest line by line, in place: every line that is not a tag or a
// releaseId is preserved byte for byte, so the file's shape never has to be
// reconstructed from a slice.
//'x
const out = [];
let cur = null;
let curTag = null;
for (const line of manLines) {
  let m = line.match(/^\s*- service: (\S+)$/);
  if (m) { cur = m[1]; curTag = null; out.push(line); continue; }
  m = line.match(/^(\s*tag: )(\S+)$/);
  if (m && cur) {
    curTag = tagOf[cur] || m[2];
    out.push(m[1] + curTag);
    continue;
  }
  m = line.match(/^(\s*releaseId: )(\S+)$/);
  if (m && cur && curTag) {
    out.push(m[1] + sha256(cur + ":" + curTag).slice(0, 10));
    continue;
  }
  out.push(line);
}
const nextMan = out.join("\n");
if (nextMan === manText) throw new Error("manifest rewrite produced no change");
await tools.stageEdit({ path: "deploy/manifest.yaml", edits: [{ old: manText, new: nextMan }] });
return done.length;
