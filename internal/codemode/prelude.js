// The guest prelude: the only JavaScript the model never wrote. It defines
// `tools.*`, encodes one host frame, calls the single import (`moca.invoke`)
// and decodes the answer.
//
// It runs inside QuickJS-NG, so it assumes no host global beyond the five the
// reactor installs: `__moca_alloc`, `__moca_free`, `__moca_store`,
// `__moca_slice` and `__moca_invoke`. The first four are thin views over the
// module's linear memory; the fifth IS the `moca.invoke` import.
//
// Two independent facts shape it (plan v13, R69):
//   - the SHAPE follows the session's stored prelude variant (NetworkPrelude):
//     a resumed session's script sees the API its system prompt promised;
//   - ADMISSION follows the current effective switch (Network): a session that
//     started with network and now has it off keeps the methods and gets a
//     structured `network_disabled` refusal instead of a silent absence.
//
// The refusal is thrown as an object carrying {class:"network_disabled"} and is
// raised BEFORE any host call, so the class never travels through moca.invoke,
// is never counted as a web attempt (§Failure ABI), and the host accepts only
// this one class from a guest (R71) — every other thrown object is `script`.
//
// Placeholders (__NET_SHAPE__, __NET_LIVE__, __REQ_MAX__, __RES_MAX__) are
// substituted by prelude.go; the file is not valid on its own by design.

(function () {
  "use strict";
  var NET_SHAPE = __NET_SHAPE__;   // the stored variant: are tools.web.* defined?
  var NET_LIVE = __NET_LIVE__;     // is network admitted for THIS execution?
  var REQ_MAX = __REQ_MAX__, RES_MAX = __RES_MAX__;

  function utf8encode(s) {
    var out = [], i, c;
    for (i = 0; i < s.length; i++) {
      c = s.charCodeAt(i);
      if (c < 0x80) { out.push(c); }
      else if (c < 0x800) { out.push(0xc0 | (c >> 6), 0x80 | (c & 0x3f)); }
      else if (c >= 0xd800 && c <= 0xdbff && i + 1 < s.length) {
        var d = s.charCodeAt(i + 1);
        if (d >= 0xdc00 && d <= 0xdfff) {
          var cp = 0x10000 + ((c - 0xd800) << 10) + (d - 0xdc00);
          out.push(0xf0 | (cp >> 18), 0x80 | ((cp >> 12) & 0x3f), 0x80 | ((cp >> 6) & 0x3f), 0x80 | (cp & 0x3f));
          i++;
        } else {
          out.push(0xef, 0xbf, 0xbd); // lone high surrogate -> U+FFFD
        }
      } else if (c >= 0xd800 && c <= 0xdfff) {
        out.push(0xef, 0xbf, 0xbd);
      } else {
        out.push(0xe0 | (c >> 12), 0x80 | ((c >> 6) & 0x3f), 0x80 | (c & 0x3f));
      }
    }
    return out;
  }

  function utf8decode(bytes, n) {
    var s = "", i = 0, c, c2, c3, c4, cp;
    while (i < n) {
      c = bytes[i++];
      if (c < 0x80) {
        s += String.fromCharCode(c);
      } else if (c < 0xe0) {
        c2 = bytes[i++];
        s += String.fromCharCode(((c & 0x1f) << 6) | (c2 & 0x3f));
      } else if (c < 0xf0) {
        c2 = bytes[i++]; c3 = bytes[i++];
        s += String.fromCharCode(((c & 0x0f) << 12) | ((c2 & 0x3f) << 6) | (c3 & 0x3f));
      } else {
        c2 = bytes[i++]; c3 = bytes[i++]; c4 = bytes[i++];
        cp = (((c & 0x07) << 18) | ((c2 & 0x3f) << 12) | ((c3 & 0x3f) << 6) | (c4 & 0x3f)) - 0x10000;
        s += String.fromCharCode(0xd800 + (cp >> 10), 0xdc00 + (cp & 0x3ff));
      }
    }
    return s;
  }

  var resPtr = __moca_alloc(RES_MAX);

  // call sends one frame and returns the decoded reply. Exactly one script is
  // in flight per process, so the request and response buffers can be reused
  // without a lock: the guest reads the answer out before the next call.
  function call(op, name, args) {
    var req = utf8encode(JSON.stringify({ op: op, name: name, args: args === undefined ? null : args }));
    if (req.length > REQ_MAX) { throw { class: "limit", message: "request too large" }; }
    var reqPtr = __moca_alloc(req.length);
    for (var i = 0; i < req.length; i++) { __moca_store(reqPtr + i, req[i]); }
    var n = __moca_invoke(reqPtr, req.length, resPtr, RES_MAX);
    __moca_free(reqPtr);
    if (n === -1) { throw { class: "limit", message: "response too large" }; }
    if (n < 0) { throw { class: "script", message: "host error" }; }
    return JSON.parse(utf8decode(__moca_slice(resPtr, n), n));
  }

  // inspect runs one workspace inspection: the host answers with the payload
  // plus complete/reasons, or with an error object the prelude rethrows
  // unchanged (its class is already the classified one).
  function inspect(name, args) {
    var r = call("inspect", name, args);
    if (r.error) { throw r.error; }
    return r.value;
  }

  // sha256(text) → lowercase hex. Pure JavaScript on purpose: the sandbox has no
  // host crypto, and deriving a value from file content (a build fingerprint, a
  // lockfile digest, a manifest id) is ordinary code work that must not need a
  // host capability. No I/O, no state, nothing in the capability table.
  var K = [
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2];

  function rr(x, n) { return (x >>> n) | (x << (32 - n)); }

  function sha256(text) {
    var b = utf8encode(String(text)), n = b.length, i, t;
    var bits = n * 8;
    b.push(0x80);
    while ((b.length % 64) !== 56) { b.push(0); }
    b.push(0, 0, 0, 0, (bits >>> 24) & 255, (bits >>> 16) & 255, (bits >>> 8) & 255, bits & 255);
    var H = [0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
             0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19];
    var w = new Array(64);
    for (i = 0; i < b.length; i += 64) {
      for (t = 0; t < 16; t++) {
        w[t] = (b[i + t * 4] << 24) | (b[i + t * 4 + 1] << 16) | (b[i + t * 4 + 2] << 8) | b[i + t * 4 + 3];
      }
      for (t = 16; t < 64; t++) {
        var x = w[t - 15], y = w[t - 2];
        var s0 = rr(x, 7) ^ rr(x, 18) ^ (x >>> 3);
        var s1 = rr(y, 17) ^ rr(y, 19) ^ (y >>> 10);
        w[t] = (w[t - 16] + s0 + w[t - 7] + s1) | 0;
      }
      var a = H[0], bb = H[1], c = H[2], d = H[3], e = H[4], f = H[5], g = H[6], h = H[7];
      for (t = 0; t < 64; t++) {
        var S1 = rr(e, 6) ^ rr(e, 11) ^ rr(e, 25);
        var ch = (e & f) ^ (~e & g);
        var t1 = (h + S1 + ch + K[t] + w[t]) | 0;
        var S0 = rr(a, 2) ^ rr(a, 13) ^ rr(a, 22);
        var maj = (a & bb) ^ (a & c) ^ (bb & c);
        var t2 = (S0 + maj) | 0;
        h = g; g = f; f = e; e = (d + t1) | 0; d = c; c = bb; bb = a; a = (t1 + t2) | 0;
      }
      H[0] = (H[0] + a) | 0; H[1] = (H[1] + bb) | 0; H[2] = (H[2] + c) | 0; H[3] = (H[3] + d) | 0;
      H[4] = (H[4] + e) | 0; H[5] = (H[5] + f) | 0; H[6] = (H[6] + g) | 0; H[7] = (H[7] + h) | 0;
    }
    var out = "";
    for (i = 0; i < 8; i++) {
      out += ("0000000" + ((H[i] >>> 0).toString(16))).slice(-8);
    }
    return out;
  }
  globalThis.sha256 = sha256;

  var tools = {};
  Object.defineProperty(tools, "read", { value: function (a) { return inspect("read", a || {}); }, enumerable: true });
  Object.defineProperty(tools, "search", { value: function (a) { return inspect("search", a || {}); }, enumerable: true });
  Object.defineProperty(tools, "ls", { value: function (a) { return inspect("ls", a || {}); }, enumerable: true });
  // glob names the paths a script needs before it can loop over them: without it
  // "one program over all the files" is only writable when the names are known.
  Object.defineProperty(tools, "glob", { value: function (a) { return inspect("glob", a || {}); }, enumerable: true });

  // __WEB_BEGIN__ the network variant's methods: cut from the source when the
  // stored variant is workspace-only (prelude.go), so the guest never sees them.
  if (NET_SHAPE) {
    // Raised before any host call, so the class never travels through
    // moca.invoke and never counts as a web attempt (§Failure ABI).
    function webRefused() {
      throw { class: "network_disabled", message: "network is not enabled for this session" };
    }

    var web = {};
    Object.defineProperty(web, "fetch", {
      enumerable: true,
      value: function (a) { if (!NET_LIVE) { webRefused(); } return inspect("web_fetch", a || {}); }
    });
    Object.defineProperty(web, "search", {
      enumerable: true,
      value: function (a) { if (!NET_LIVE) { webRefused(); } return inspect("web_search", a || {}); }
    });
    Object.defineProperty(tools, "web", { value: web, enumerable: true });
  }
  // __WEB_END__

  // __EDITS_BEGIN__ the writable workspace half: present only when the session
  // declared workspace + workspaceEdits (prelude.go cuts this block otherwise).
  if (__EDITS__) {
    Object.defineProperty(tools, "stageEdit", {
      enumerable: true,
      value: function (a) { return inspect("stageEdit", a || {}); }
    });
  }
  // __EDITS_END__

  // An unknown property throws (§Payloads): a typo must not silently do nothing,
  // and it must not reach the host either.
  Object.preventExtensions(tools);
  globalThis.tools = new Proxy(tools, {
    get: function (t, k) {
      if (typeof k !== "string") { return undefined; }
      if (Object.prototype.hasOwnProperty.call(t, k)) { return t[k]; }
      throw { class: "script", message: "unknown tools." + k + " (available: " + Object.keys(t).join(", ") + ")" };
    }
  });

  // Accepted output events: the host keeps them even when the script later
  // traps, which is why they are host-recorded rather than returned.
  globalThis.text = function (v) { call("text", null, v); return null; };
  globalThis.log = function (v) { call("log", null, String(v)); return null; };
})();
