# procsearch

Finds running processes and loaded modules by name. It is a clean Go reimplementation
of the "search" logic recovered from `../test.exe`. Standard library only.

```
make            # fmt, vet (linux/windows/darwin), test, build
make windows    # bin/procsearch.exe

bin/procsearch ps zsh                      # list processes named exactly "zsh"
bin/procsearch ps -match contains code     # case-insensitive substring
bin/procsearch ps -first zsh               # print just the PID
bin/procsearch mods -pid 1234 libc.so.6    # list matching modules in pid 1234
bin/procsearch mods -pid 1234 -first LIBC.SO.6   # print just the base address
```

Exit status: `0` found, `1` not found, `2` usage/system error (e.g. permission denied).

## Layout

| Path | Role |
|---|---|
| `internal/search` | OS-independent matching: `Mode` (exact / fold / contains), `All`, `Best`, `ErrNotFound` |
| `internal/proc/proc.go` | `FindProcessID`, `FindProcesses`, `FindModuleBase`, `FindModules` |
| `internal/proc/proc_linux.go` | snapshots from `/proc/<pid>/stat`, `exe`, `maps` |
| `internal/proc/proc_windows.go` | snapshots from Toolhelp32 (`Process32FirstW/NextW`, `Module32FirstW/NextW`) |
| `cmd/procsearch` | CLI |

## Evidence: how search works in `test.exe`

Static analysis only; the binary was never executed.
SHA-256 `1564ba084b73aee3bafbdaf2491d9e05fe03a9240ea0aa92f31190a71c2db459`, PE32+ x86-64, Go,
built from `C:/Projects/cs2/cs2go/cs2go/` (paths recovered from the Go `pclntab`).

There is **no signature / pattern scan**. Game offsets are read from `offsets.json`.
"Search" is two linear lookups over a Toolhelp32 snapshot:

**`main.findProcessId`**: `memory.go:20`, `0x1400fbcc0–0x1400fbf00`
```
1400fbd15  call CreateToolhelp32Snapshot
1400fbdc0  call Process32Next          ; loop; Process32First is never called
1400fbde0  call syscall.UTF16ToString  ; szExeFile -> string
1400fbded  cmp  rbx,rdx                ; length equal?
1400fbe00  call runtime.memequal       ; byte-exact, case-SENSITIVE compare
1400fbe58  lea  rax,"module not found" ; len 0x10 -> fmt.Errorf
```

**`main.getModuleBaseAddress`**: `memory.go:41`, `0x1400fbf60–0x1400fc200`
```
1400fbfb6  call CreateToolhelp32Snapshot
1400fc022  call Module32First          ; error "Module32First failed"
1400fc0c6  call syscall.UTF16ToString
1400fc0e0  call strings.EqualFold      ; case-INSENSITIVE compare
1400fc0f3  call Module32Next
1400fc0fd  lea  rax,"module not found" ; len 0x10 -> fmt.Errorf
```

**Call site in `main.main`**
```
1400fae05  lea rax,"cs2.exe"    ; mov ebx,7
1400fae11  call main.findProcessId
1400fae27  lea rbx,"client.dll" ; mov ecx,0xa
1400fae33  call main.getModuleBaseAddress
```

### Original vs. this rebuild

| Original behaviour | Here |
|---|---|
| Process loop starts at `Process32Next`, skipping the first entry | Starts with `Process32First` (covered by a test) |
| Both failures say `"module not found"` | `process "x": not found` / `module "x" in pid N: not found`, wrapping `search.ErrNotFound` |
| Process match exact, module match fold (hard-coded) | Same defaults, selectable per call: `-match exact\|fold\|contains` |
| Returns the first hit | Exact-case hit preferred, then first hit in snapshot order |
| Windows only | Linux (`/proc`) and Windows. Other OSes return a clear error |

The memory-reading and overlay parts of the original are intentionally not reproduced.

## Notes

- Linux names come from the `/proc/<pid>/exe` basename, with the 15-byte `comm` as fallback.
  A module's base is the lowest mapping of that file in `/proc/<pid>/maps`.
- Listing another user's modules needs ptrace permission (same user or root). Otherwise you get `permission denied`.
- The Windows code is vetted and cross-compiled but has not been run on a Windows host.
