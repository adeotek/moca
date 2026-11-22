package codemode

// The ABI gate must read the module's IMPORT SECTION, not wazero's function
// definitions: for an imported function wazero reports the module name from the
// NAME SECTION, which wasm-ld sets to the output file's basename. Reading that
// makes every artifact look like it imports from `qjs-moca-reactor.wasm`, which
// is what this gate must never accept and must never falsely reject either.
//
// The parser is deliberately minimal: wazero has already validated the module
// (CompileModule), so this only needs to walk section 2 and skip descriptors.

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// wasmImport is one entry of the import section.
type wasmImport struct {
	Module string
	Name   string
	Kind   byte // 0 function, 1 table, 2 memory, 3 global, 4 tag
}

// wasmImports returns section 2's entries, or nil when the module imports
// nothing.
func wasmImports(wasm []byte) ([]wasmImport, error) {
	if len(wasm) < 8 || binary.LittleEndian.Uint32(wasm[:4]) != 0x6d736100 {
		return nil, errors.New("not a wasm module (bad magic)")
	}
	if binary.LittleEndian.Uint32(wasm[4:8]) != 1 {
		return nil, errors.New("unsupported wasm version")
	}
	for i := 8; i < len(wasm); {
		id := wasm[i]
		i++
		size, n, err := uleb(wasm[i:])
		if err != nil {
			return nil, fmt.Errorf("section size: %w", err)
		}
		i += n
		if int(size) > len(wasm)-i {
			return nil, errors.New("section overruns the module")
		}
		body := wasm[i : i+int(size)]
		i += int(size)
		if id == 2 {
			return parseImports(body)
		}
	}
	return nil, nil
}

func parseImports(b []byte) ([]wasmImport, error) {
	count, n, err := uleb(b)
	if err != nil {
		return nil, fmt.Errorf("import count: %w", err)
	}
	b = b[n:]
	out := make([]wasmImport, 0, count)
	for k := uint32(0); k < count; k++ {
		var mod, name string
		if mod, b, err = wasmString(b); err != nil {
			return nil, fmt.Errorf("import %d module: %w", k, err)
		}
		if name, b, err = wasmString(b); err != nil {
			return nil, fmt.Errorf("import %d name: %w", k, err)
		}
		if len(b) < 1 {
			return nil, fmt.Errorf("import %d: truncated kind", k)
		}
		kind := b[0]
		if b, err = skipDescriptor(kind, b[1:]); err != nil {
			return nil, fmt.Errorf("import %d (%s.%s): %w", k, mod, name, err)
		}
		out = append(out, wasmImport{Module: mod, Name: name, Kind: kind})
	}
	return out, nil
}

// skipDescriptor advances past an import's type descriptor.
func skipDescriptor(kind byte, b []byte) ([]byte, error) {
	switch kind {
	case 0: // function: type index
		_, n, err := uleb(b)
		if err != nil {
			return nil, err
		}
		return b[n:], nil
	case 1: // table: element type, then limits
		if len(b) < 1 {
			return nil, errors.New("truncated table type")
		}
		return skipLimits(b[1:])
	case 2: // memory: limits
		return skipLimits(b)
	case 3: // global: value type + mutability
		if len(b) < 2 {
			return nil, errors.New("truncated global type")
		}
		return b[2:], nil
	case 4: // tag: attribute byte + type index
		if len(b) < 1 {
			return nil, errors.New("truncated tag type")
		}
		_, n, err := uleb(b[1:])
		if err != nil {
			return nil, err
		}
		return b[1+n:], nil
	default:
		return nil, fmt.Errorf("unknown import kind %d", kind)
	}
}

// skipLimits advances past a limits record: flags, min, and max when bit 0 is set.
func skipLimits(b []byte) ([]byte, error) {
	if len(b) < 1 {
		return nil, errors.New("truncated limits")
	}
	flags := b[0]
	b = b[1:]
	_, n, err := uleb(b)
	if err != nil {
		return nil, err
	}
	b = b[n:]
	if flags&0x01 != 0 {
		_, n, err = uleb(b)
		if err != nil {
			return nil, err
		}
		b = b[n:]
	}
	return b, nil
}

func wasmString(b []byte) (string, []byte, error) {
	n, k, err := uleb(b)
	if err != nil {
		return "", nil, err
	}
	if int(n) > len(b)-k {
		return "", nil, errors.New("string overruns its section")
	}
	return string(b[k : k+int(n)]), b[k+int(n):], nil
}

// uleb decodes an unsigned LEB128 value.
func uleb(b []byte) (uint32, int, error) {
	var v uint32
	for i := 0; i < len(b) && i < 5; i++ {
		c := b[i]
		v |= uint32(c&0x7f) << (7 * uint(i))
		if c&0x80 == 0 {
			return v, i + 1, nil
		}
	}
	return 0, 0, errors.New("bad LEB128")
}
