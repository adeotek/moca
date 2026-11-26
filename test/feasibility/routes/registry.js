// The codemode route for changeset-registry: one program for every page and
// the registry, with the gofmt alignment computed rather than shelled out to.
const raw = await tools.read({ path: "plugins/data.json" });
const data = JSON.parse(raw.lines.map(l => l.text).join("\n"));
const byName = {};
for (const p of data.plugins) byName[p.name] = p;
const pages = await tools.glob({ pattern: "capabilities/*.go" });
const done = [];
for (const path of pages.paths) {
  const name = path.split("/")[1].replace(/\.go$/, "");
  if (name === "spec") continue;
  const p = byName[name];
  if (!p || !p.enabled) continue;
  const hex = sha256(name + "|" + p.kind + "|" + p.endpoint);
  const text = (await tools.read({ path })).lines.map(l => l.text).join("\n");
  const old = text.match(/^\tDigest:   ".*",$/m)[0];
  await tools.stageEdit({ path, edits: [
    { old, new: '\tDigest:   "' + hex.slice(0, 8) + '",' }] });
  done.push([name, p.kind, hex.slice(0, 6)]);
}
done.sort((a, b) => (a[0] < b[0] ? -1 : 1));
const width = Math.max(...done.map(d => ('"' + d[0] + '":').length));
const rows = done.map(([n, k, h]) => '\t' + ('"' + n + '":').padEnd(width) + ' "' + k + "@" + h + '",').join("\n");
await tools.stageEdit({ path: "plugins/registry.go", edits: [
  { old: (await tools.read({ path: "plugins/registry.go" })).lines.map(l => l.text).join("\n")
      .split("\n").filter(l => l.startsWith("\t\"")).join("\n"), new: rows }] });
return done.length;
