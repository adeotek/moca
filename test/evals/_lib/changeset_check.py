"""Change-set graders: a many-file, data-dependent edit, graded on bytes and on
the transcript (codemode workspace-changesets plan v7, phase 1).

Two scenarios share this checker because they share a contract, not a fixture:

- `changeset-pins` — releases.json names new image versions; every released
  service's `service.yaml` and its entry in `deploy/manifest.yaml` must carry the
  new tag, and nothing else may move.
- `changeset-registry` — plugins/data.json changes an endpoint (and usually a
  kind and a timeout) for every enabled plugin; each `capabilities/<name>.go`
  page must match it, and `plugins/registry.go` must list exactly the enabled
  plugins, sorted.

What it proves, in this order (a failure names the thing that failed):

1. The tree. Every file the run was allowed to touch is checked byte-for-byte
   against the expected image, derived here from the fixture's data file — not
   from the run's own output. Every other fixture file must be byte-identical,
   and no file may appear that the fixture did not have (`verify.py`, the data
   file and the prompt's target files included). A global rewrite that loses a
   comment, a reformat, a stray scratch file: each fails here.
2. The verification. The transcript must show the requested check succeeding,
   with no typed mutation after it: `python3 verify.py` (or a path to it) as a
   `shell` call whose result is not an error. A "verified" claim without the call
   is not a verification, and a verify followed by an edit proves nothing — but
   *how* the edit was made is not the grader's business, so a shell script that
   rewrites the files and then verifies passes.
3. The answer. The two closing lines are parsed and compared with the counts
   the fixture implies. Prose elsewhere is ignored; the exact lines are the
   contract.

The session is a JSONL transcript: `tool_use` entries carry
`toolUse.call.{id,name,input}`, `tool_result` entries carry
`toolResult.{callId,content,isError}`.
"""

import json
import os
import pathlib
import re
import sys

MUTATIONS = {"edit", "write", "codemode"}


# --------------------------------------------------------------------- helpers

def fail(msg):
    print(msg)
    sys.exit(1)


def read(path):
    return pathlib.Path(path).read_text()


def walk(root, skip=(".git", ".moca")):
    root = pathlib.Path(root)
    for p in sorted(root.rglob("*")):
        if p.is_dir():
            continue
        rel = p.relative_to(root)
        if any(part in skip for part in rel.parts):
            continue
        yield rel.as_posix(), p


def tool_calls(session):
    """(uses, results): uses is [(callId, name, input)] in order; results maps
    callId -> toolResult object. Torn lines are skipped like the harness does."""
    uses, results = [], {}
    for line in open(session):
        line = line.strip()
        if not line:
            continue
        try:
            e = json.loads(line)
        except json.JSONDecodeError:
            continue
        if e.get("type") == "tool_use":
            call = (e.get("toolUse") or {}).get("call") or {}
            uses.append((call.get("id", ""), call.get("name", ""), call.get("input") or {}))
        elif e.get("type") == "tool_result":
            r = e.get("toolResult") or {}
            results[r.get("callId", "")] = r
    return uses, results


def is_error(result):
    return bool(result.get("isError") or result.get("is_error"))


def session_problems(session):
    """The transcript half: a successful verify.py run, and no typed mutation
    after it.

    The verdict deliberately does not care *how* the edits were made: a shell
    script that rewrites the files and then verifies is a correct ordinary
    solution and must pass, so a `shell` call counts as the mutation. What the
    rule rejects is the ordering — a verify followed by an edit is a verify of
    a tree that no longer exists.
    """
    uses, results = tool_calls(session)
    if not uses:
        return "the transcript has no tool calls"
    verified = -1
    for i, (cid, name, inp) in enumerate(uses):
        if name != "shell":
            continue
        if "verify.py" not in str(inp.get("command", "")):
            continue
        if is_error(results.get(cid) or {}):
            continue  # a failing verify is not a verification
        verified = i
    if verified < 0:
        return "no successful `verify.py` run in the transcript"
    for _, name, _ in uses[verified + 1:]:
        if name in MUTATIONS:
            return ("`verify.py` ran, then %s changed the tree: the verification "
                    "must come after the last mutation" % name)
    if verified == 0:
        return "`verify.py` ran before anything was changed"
    return None


def final_assistant_text(session):
    last = ""
    for line in open(session):
        line = line.strip()
        if not line:
            continue
        try:
            e = json.loads(line)
        except json.JSONDecodeError:
            continue
        if e.get("type") != "message":
            continue
        m = e.get("message") or {}
        if m.get("role") != "assistant":
            continue
        text = "\n".join(c.get("text", "") for c in m.get("content", [])
                         if c.get("type") == "text")
        if text.strip():
            last = text
    return last


def tree_and_answer(scen_dir, work, session, expectations):
    """Shared tail: extra-file refusal, session check, answer lines."""
    fixture_files = {rel for rel, _ in walk(pathlib.Path(scen_dir) / "fixture")}
    for rel, _ in walk(work):
        if rel not in fixture_files and rel not in expectations.written:
            fail(f"the run left a file the task did not allow: {rel}")

    problem = session_problems(session)
    if problem:
        fail(problem)

    text = final_assistant_text(session)
    if not text:
        fail("no final answer in prose")
    got = expectations.answer(text)
    if got:
        fail(got)


# ----------------------------------------------------------------------- pins

class PinsExpectations:
    """Expected image for changeset-pins, derived from releases.json.

    The rollout is discovered, not listed: a service is in it when it appears
    in stages[rollout.stage], has a release, and its own service.yaml declares
    at least two replicas. Its tag is the version plus a seven-hex fingerprint
    of salt:name:replicas, its pin block carries ten times its replicas, and
    the manifest's releaseId is a ten-hex fingerprint of name + tag.
    """

    ENTRY = ("  - service: {name}\n    tag: {tag}\n    releaseId: {rid}\n")

    def __init__(self, scen_dir):
        import hashlib
        self.scen = pathlib.Path(scen_dir)
        self.fixture_files = {rel for rel, _ in walk(self.scen / "fixture")}
        d = json.loads(read(self.scen / "fixture" / "releases.json"))
        self.d = d
        self.rel = d["services"]
        stage = d["rollout"]["stage"]
        self.salt = d["salt"]
        in_stage = set(d["stages"][stage])
        self.targets = sorted(n for n, r in self.rel.items()
                              if n in in_stage and r["replicas"] >= 2)
        self.legacy = d.get("legacy", [])
        self.written = {f"services/{n}/service.yaml" for n in self.targets} | {"deploy/manifest.yaml"}

    def fp(self, *parts, n):
        import hashlib
        return hashlib.sha256(":".join(parts).encode()).hexdigest()[:n]

    def tag(self, name):
        return f"{self.rel[name]['version']}-{self.fp(self.salt, name, str(self.rel[name]['replicas']), n=7)}"

    def expected(self, rel, want):
        src = read(self.scen / "fixture" / rel)
        names = [n for n in self.rel] + self.legacy
        if rel == "deploy/manifest.yaml":
            entries = []
            for name in names:
                if name in self.targets:
                    tag = self.tag(name)
                    rid = self.fp(name, tag, n=10)
                elif name in self.rel:
                    tag = self.rel[name]["version"]
                    rid = self.fp(name, tag, n=10)
                else:
                    tag = "0.9.4"
                    rid = self.fp(name, tag, n=10)
                entries.append(self.ENTRY.format(name=name, tag=tag, rid=rid))
            head = src[:src.index("services:\n") + len("services:\n")]
            return head + "".join(entries)
        m = re.match(r"services/([^/]+)/service\.yaml$", rel)
        if not m or m.group(1) not in self.targets:
            return None
        name = m.group(1)
        r = self.rel[name]
        tag = self.tag(name)
        out, n = re.subn(r"^(image: ghcr\.io/acme/[a-z0-9-]+:)\S+$",
                         lambda mm: mm.group(1) + tag, src, count=1, flags=re.M)
        if n != 1:
            fail(f"fixture defect: no image line in {rel}")
        return out + f"pin:\n  tag: {tag}\n  weight: {r['replicas'] * 10}\n"

    def answer(self, text):
        one = re.search(r"^updated=(\d+) services,(\d+) manifest tags$", text, re.M)
        two = re.search(r"^untouched=(\d+) services,(\d+) legacy entries$", text, re.M)
        if not one or not two:
            return "the answer is missing the two required lines (updated=… / untouched=…)"
        total = sum(1 for p in (self.scen / "fixture" / "services").iterdir() if p.is_dir())
        want = (len(self.targets), len(self.targets), total - len(self.targets), len(self.legacy))
        got = (int(one.group(1)), int(one.group(2)), int(two.group(1)), int(two.group(2)))
        if got != want:
            return f"the answer says {got}, the fixture implies {want}"
        return None


# ------------------------------------------------------------------- registry

class RegistryExpectations:
    """Expected bytes for changeset-registry.

    The target set is the pages that exist, filtered by data.json: an enabled
    plugin with a page is regenerated with a digest derived from its name, kind
    and endpoint; an enabled plugin with no page is not created, and a disabled
    plugin's page stays byte-identical.
    """

    HEADER = ("package plugins\n\n"
              "// registry maps an enabled plugin to the kind its factory expects.\n"
              "// Generated from plugins/data.json — regenerate, do not hand-edit.\n"
              "var registry = map[string]string{\n")
    PAGE = ("package capabilities\n\n"
            "// {ident} is generated from plugins/data.json — edit the data, not this file.\n"
            "var {ident} = Spec{{\n"
            "\tName:     \"{name}\",\n"
            "\tKind:     \"{kind}\",\n"
            "\tEndpoint: \"{endpoint}\",\n"
            "\tTimeout:  {timeout},\n"
            "\tDigest:   \"{digest}\",\n"
            "}}\n")

    def __init__(self, scen_dir):
        self.scen = pathlib.Path(scen_dir)
        self.fixture_files = {rel for rel, _ in walk(self.scen / "fixture")}
        self.data = {p["name"]: p for p in
                     json.loads(read(self.scen / "fixture" / "plugins" / "data.json"))["plugins"]}
        pages = {p.stem for p in (self.scen / "fixture" / "capabilities").glob("*.go") if p.stem != "spec"}
        self.targets = sorted(n for n in pages if n in self.data and self.data[n]["enabled"])
        self.disabled = sorted(n for n in pages if n in self.data and not self.data[n]["enabled"])
        self.written = {f"capabilities/{n}.go" for n in self.targets} | {"plugins/registry.go"}

    def digest(self, name, n=8):
        import hashlib
        p = self.data[name]
        return hashlib.sha256(f"{name}|{p['kind']}|{p['endpoint']}".encode()).hexdigest()[:n]

    def expected(self, rel, want):
        if rel == "plugins/registry.go":
            pairs = sorted((n, self.data[n]["kind"], self.digest(n, 6)) for n in self.targets)
            width = max(len(f'"{n}":') for n, _, _ in pairs)
            rows = "\n".join('\t' + (f'"{n}":'.ljust(width)) + f' "{k}@{d}",' for n, k, d in pairs)
            return self.HEADER + rows + "\n}\n"
        m = re.match(r"capabilities/([^/]+)\.go$", rel)
        if not m or m.group(1) not in self.targets:
            return None
        p = self.data[m.group(1)]
        return self.PAGE.format(ident=m.group(1).replace("-", " ").title().replace(" ", ""),
                                name=p["name"], kind=p["kind"], endpoint=p["endpoint"],
                                timeout=p["timeoutMs"], digest=self.digest(p["name"]))

    def answer(self, text):
        one = re.search(r"^updated=(\d+) plugin pages,(\d+) registry rows$", text, re.M)
        two = re.search(r"^untouched=(\d+) disabled plugins$", text, re.M)
        if not one or not two:
            return "the answer is missing the two required lines (updated=… / untouched=…)"
        want = (len(self.targets), len(self.targets), len(self.disabled))
        got = (int(one.group(1)), int(one.group(2)), int(two.group(1)))
        if got != want:
            return f"the answer says {got}, the fixture implies {want}"
        return None


# --------------------------------------------------------------------- driver

def check(scenario, work, session, scen_dir):
    if not session or not os.path.isfile(session):
        fail("no session transcript: a two-turn scenario is graded through one")
    exp = (PinsExpectations if scenario == "changeset-pins" else RegistryExpectations)(scen_dir)
    fixture = pathlib.Path(scen_dir) / "fixture"
    seen = False
    for rel, path in walk(work):
        if rel not in exp.fixture_files and rel not in exp.written:
            fail(f"the run left a file the task did not allow: {rel}")
        want = exp.expected(rel, rel in exp.written)
        have = path.read_text(errors="replace")
        src = read(fixture / rel) if (fixture / rel).is_file() else None
        if want is None:
            if src is not None and have != src:
                fail(f"{rel} changed, and the task did not name it")
            continue
        seen = True
        if have != want:
            first = next((i + 1 for i, (a, b) in enumerate(zip(have.splitlines(), want.splitlines())) if a != b), None)
            fail(f"{rel} does not hold the expected content"
                 + (f" (first difference at line {first})" if first else " (length differs)"))
    if not seen:
        fail("no target file was touched")
    tree_and_answer(scen_dir, work, session, exp)


def main(argv):
    if len(argv) != 5:
        fail("usage: changeset_check.py <scenario> <workdir> <session> <scenario-dir>")
    check(*argv[1:])
    print("pass")


if __name__ == "__main__":
    main(sys.argv)
