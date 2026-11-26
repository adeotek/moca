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
  if (EDITS) {
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
