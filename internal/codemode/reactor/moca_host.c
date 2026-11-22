/* moca_host.c — the reactor's C half.
 *
 * One QuickJS-NG runtime per guest instance, one script per qjs_run call, one
 * import (`moca.invoke`). Everything the guest can reach is installed here:
 * five thin memory helpers plus the invoke call. There is no filesystem and no
 * socket: wasi-libc is linked, but abi.json forbids those imports and
 * VerifyModule fails a build that acquires one.
 *
 * ABI (internal/codemode/abi.json):
 *   export qjs_init(heap, stack) -> 0 ok / -1 fail
 *   export qjs_run(src_ptr, src_len, out_ptr, out_cap, out_len_ptr) -> status
 *   export qjs_destroy()
 *   export moca_alloc(n) -> ptr
 *   export moca_free(ptr)
 *   import moca.invoke(req_ptr, req_len, res_ptr, res_cap) -> len / -1 / -2
 *
 * qjs_run status: 0 ok, 1 syntax, 2 throw, 3 oom, 4 job limit, 5 output too
 * large, 6 internal. out_len is always written.
 */

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#include "quickjs.h"

#define EXPORT(name) __attribute__((export_name(#name), visibility("default")))
#define IMPORT(module, name) __attribute__((import_module(#module), import_name(#name)))

IMPORT(moca, invoke)
extern int32_t moca_invoke(int32_t req_ptr, int32_t req_len, int32_t res_ptr, int32_t res_cap);

static JSRuntime *rt = NULL;
static JSContext *ctx = NULL;

/* ---- imports exposed to JavaScript -------------------------------------- */
/* The prelude does the UTF-8 work; these are byte-level views over linear
 * memory so nothing here has to understand strings. */

static JSValue js_alloc(JSContext *c, JSValueConst this_val, int argc, JSValueConst *argv) {
    int32_t n = 0;
    if (JS_ToInt32(c, &n, argv[0]) < 0 || n < 0) return JS_EXCEPTION;
    void *p = malloc((size_t)n > 0 ? (size_t)n : 1);
    return JS_NewInt32(c, (int32_t)(intptr_t)p);
}

static JSValue cb_free(JSContext *c, JSValueConst this_val, int argc, JSValueConst *argv) {
    int32_t p = 0;
    if (JS_ToInt32(c, &p, argv[0]) < 0) return JS_EXCEPTION;
    if (p) free((void *)(intptr_t)p);
    return JS_UNDEFINED;
}

static JSValue js_store(JSContext *c, JSValueConst this_val, int argc, JSValueConst *argv) {
    int32_t p = 0, b = 0;
    if (JS_ToInt32(c, &p, argv[0]) < 0) return JS_EXCEPTION;
    if (JS_ToInt32(c, &b, argv[1]) < 0) return JS_EXCEPTION;
    ((uint8_t *)(intptr_t)p)[0] = (uint8_t)(b & 0xff);
    return JS_UNDEFINED;
}

/* js_slice returns a plain Array of numbers: the prelude decodes UTF-8 from it,
 * which keeps the C side free of string-length subtleties. */
static JSValue js_slice(JSContext *c, JSValueConst this_val, int argc, JSValueConst *argv) {
    int32_t p = 0, n = 0;
    if (JS_ToInt32(c, &p, argv[0]) < 0) return JS_EXCEPTION;
    if (JS_ToInt32(c, &n, argv[1]) < 0 || n < 0) return JS_EXCEPTION;
    JSValue arr = JS_NewArray(c);
    if (JS_IsException(arr)) return arr;
    const uint8_t *base = (const uint8_t *)(intptr_t)p;
    for (int32_t i = 0; i < n; i++) {
        if (JS_SetPropertyUint32(c, arr, (uint32_t)i, JS_NewInt32(c, base[i])) < 0) {
            JS_FreeValue(c, arr);
            return JS_EXCEPTION;
        }
    }
    return arr;
}

static JSValue js_invoke(JSContext *c, JSValueConst this_val, int argc, JSValueConst *argv) {
    int32_t req_ptr = 0, req_len = 0, res_ptr = 0, res_cap = 0;
    if (JS_ToInt32(c, &req_ptr, argv[0]) < 0) return JS_EXCEPTION;
    if (JS_ToInt32(c, &req_len, argv[1]) < 0) return JS_EXCEPTION;
    if (JS_ToInt32(c, &res_ptr, argv[2]) < 0) return JS_EXCEPTION;
    if (JS_ToInt32(c, &res_cap, argv[3]) < 0) return JS_EXCEPTION;
    return JS_NewInt32(c, moca_invoke(req_ptr, req_len, res_ptr, res_cap));
}

/* ---- the bounded output buffer ----------------------------------------- */

typedef struct {
    char *buf;
    size_t cap;
    size_t len;
    int overflow;
} outbuf;

static void ob_raw(outbuf *o, const char *s, size_t n) {
    if (o->overflow) return;
    if (o->len + n > o->cap) { o->overflow = 1; return; }
    memcpy(o->buf + o->len, s, n);
    o->len += n;
}

static void ob_str(outbuf *o, const char *s) { ob_raw(o, s, strlen(s)); }

/* ob_json_string escapes into the buffer: the envelope is the only place a
 * guest string reaches the host, so this is the one escaping path. */
static void ob_json_string(outbuf *o, const char *s, size_t n) {
    ob_raw(o, "\"", 1);
    for (size_t i = 0; i < n; i++) {
        unsigned char c = (unsigned char)s[i];
        char tmp[8];
        switch (c) {
        case '"':  ob_raw(o, "\\\"", 2); break;
        case '\\': ob_raw(o, "\\\\", 2); break;
        case '\n': ob_raw(o, "\\n", 2); break;
        case '\r': ob_raw(o, "\\r", 2); break;
        case '\t': ob_raw(o, "\\t", 2); break;
        default:
            if (c < 0x20) {
                snprintf(tmp, sizeof tmp, "\\u%04x", c);
                ob_raw(o, tmp, 6);
            } else {
                ob_raw(o, (const char *)&c, 1);
            }
        }
    }
    ob_raw(o, "\"", 1);
}

/* ---- exception classification ------------------------------------------ */

static const char *prop_cstr(JSContext *c, JSValueConst obj, const char *name) {
    JSValue v = JS_GetPropertyStr(c, obj, name);
    const char *s = NULL;
    if (!JS_IsUndefined(v) && !JS_IsException(v)) s = JS_ToCString(c, v);
    JS_FreeValue(c, v);
    return s; /* caller frees with JS_FreeCString */
}

/* status_of names the class qjs_run reports. The Go side applies the R71
 * allowlist anyway; this only distinguishes a parse error from a throw so the
 * outer result can say `syntax` without a second round trip. */
static int status_of(JSContext *c, JSValueConst exc, const char **class_out, char **msg_out, int *line_out) {
    *class_out = "script";
    *msg_out = NULL;
    *line_out = 0;
    if (JS_IsUndefined(exc) || JS_IsNull(exc)) return 2;

    const char *name = prop_cstr(c, exc, "name");
    const char *msg = prop_cstr(c, exc, "message");
    if (msg) {
        *msg_out = strdup(msg);
        JS_FreeCString(c, msg);
    }
    JSValue ln = JS_GetPropertyStr(c, exc, "lineNumber");
    int32_t v = 0;
    if (JS_IsNumber(ln) && JS_ToInt32(c, &v, ln) == 0) *line_out = (int)v;
    JS_FreeValue(c, ln);
    int syntax = 0, oom = 0;
    if (name) {
        syntax = strcmp(name, "SyntaxError") == 0;
        /* quickjs reports the memory limit as InternalError("out of memory"):
         * it is the ABI's `oom` status, not a script bug. */
        oom = strcmp(name, "InternalError") == 0 && *msg_out && strstr(*msg_out, "out of memory") != NULL;
        JS_FreeCString(c, name);
    }
    if (oom) return 3;
    if (*msg_out == NULL) *msg_out = strdup("the script failed");
    return syntax ? 1 : 2;
}

/* ---- lifecycle ---------------------------------------------------------- */

EXPORT(qjs_init)
int32_t qjs_init(int32_t heap_bytes, int32_t stack_bytes) {
    if (rt) return 0;
    rt = JS_NewRuntime();
    if (!rt) return -1;
    JS_SetMemoryLimit(rt, (size_t)(heap_bytes > 0 ? heap_bytes : (12 << 20)));
    JS_SetMaxStackSize(rt, (size_t)(stack_bytes > 0 ? stack_bytes : (1 << 20)));
    ctx = JS_NewContext(rt);
    if (!ctx) { JS_FreeRuntime(rt); rt = NULL; return -1; }

    JSValue g = JS_GetGlobalObject(ctx);
    struct { const char *n; JSCFunction *f; } fns[] = {
        {"__moca_alloc", js_alloc}, {"__moca_free", cb_free},
        {"__moca_store", js_store}, {"__moca_slice", js_slice},
        {"__moca_invoke", js_invoke},
    };
    for (size_t i = 0; i < sizeof fns / sizeof fns[0]; i++) {
        JS_SetPropertyStr(ctx, g, fns[i].n, JS_NewCFunction(ctx, fns[i].f, fns[i].n, 4));
    }
    JS_FreeValue(ctx, g);
    return 0;
}

EXPORT(qjs_destroy)
void qjs_destroy(void) {
    if (ctx) { JS_FreeContext(ctx); ctx = NULL; }
    if (rt) { JS_FreeRuntime(rt); rt = NULL; }
}

/* write_len stores the u32 the ABI's out_len_ptr points at. */
static void write_len(int32_t len_ptr, uint32_t v) {
    uint8_t *p = (uint8_t *)(intptr_t)len_ptr;
    p[0] = (uint8_t)(v & 0xff);
    p[1] = (uint8_t)((v >> 8) & 0xff);
    p[2] = (uint8_t)((v >> 16) & 0xff);
    p[3] = (uint8_t)((v >> 24) & 0xff);
}

EXPORT(qjs_run)
int32_t qjs_run(int32_t src_ptr, int32_t src_len, int32_t out_ptr, int32_t out_cap, int32_t len_ptr) {
    if (!ctx) return 6;
    const char *src = (const char *)(intptr_t)src_ptr;

    outbuf o = {.buf = (char *)(intptr_t)out_ptr, .cap = (size_t)out_cap, .len = 0, .overflow = 0};
    JSValue v = JS_Eval(ctx, src, (size_t)src_len, "<codemode>", JS_EVAL_TYPE_GLOBAL);

    int status = 0;
    const char *cls = NULL;
    char *msg = NULL;
    int line = 0;
    JSValue json = JS_UNDEFINED;

    if (JS_IsException(v)) {
        JSValue exc = JS_GetException(ctx);
        status = status_of(ctx, exc, &cls, &msg, &line);
        if (status == 3) cls = "oom";
        JS_FreeValue(ctx, exc);
        if (cls == NULL) cls = "script";
    } else {
        json = JS_JSONStringify(ctx, v, JS_UNDEFINED, JS_UNDEFINED);
        if (JS_IsException(json)) {
            JSValue e2 = JS_GetException(ctx);
            JS_FreeValue(ctx, e2);
            json = JS_UNDEFINED;
        }
    }

    ob_str(&o, "{\"value\":");
    if (JS_IsString(json)) {
        const char *s = JS_ToCString(ctx, json);
        if (s) { ob_raw(&o, s, strlen(s)); JS_FreeCString(ctx, s); }
        else ob_str(&o, "null");
    } else {
        ob_str(&o, "null");
    }
    ob_str(&o, ",\"texts\":[],\"logs\":[],\"error\":");
    if (status != 0) {
        char num[24];
        ob_str(&o, "{\"class\":");
        ob_json_string(&o, cls ? cls : "script", strlen(cls ? cls : "script"));
        ob_str(&o, ",\"message\":");
        ob_json_string(&o, msg ? msg : "the script failed", strlen(msg ? msg : "the script failed"));
        snprintf(num, sizeof num, ",\"line\":%d", line);
        ob_str(&o, num);
        ob_str(&o, "}");
    } else {
        ob_str(&o, "null");
    }
    ob_str(&o, "}");

    if (msg) free(msg);
    JS_FreeValue(ctx, json);
    JS_FreeValue(ctx, v);

    if (o.overflow) {
        write_len(len_ptr, 0);
        return 5; /* output too large: the Go side reports `limit` */
    }
    write_len(len_ptr, (uint32_t)o.len);
    return status;
}

EXPORT(moca_alloc)
void *moca_alloc(size_t n) { return malloc(n > 0 ? n : 1); }

EXPORT(moca_free)
void moca_free(void *p) { free(p); }
