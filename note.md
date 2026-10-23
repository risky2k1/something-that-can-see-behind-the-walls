The Go rebuild is in build/ and works: tests pass, it builds, and it found real processes and modules on this machine. The Windows version compiles but hasn't been run on a Windows machine.

# What "search" is in test.exe.

The exe is an external CS2 wallhack written in Go (game name cs2.exe, module client.dll). I only analysed it statically and never ran it. It has no signature or pattern scanning; the game memory offsets come from offsets.json. Its search is two simple name lookups:

Finding the game process (main.findProcessId, memory.go:20): takes a snapshot of running processes, loops with Process32Next, and compares each name byte-for-byte, so the match is case-sensitive.
Bug: it never calls Process32First, so the first entry in the list is never checked.
Finding a loaded module (main.getModuleBaseAddress, memory.go:41): loops over the process's loaded modules and compares names case-insensitively (strings.EqualFold).
Bug: both functions report the same "module not found" error, so a missing process looks like a missing module.
Function names, addresses and source line numbers come from Go's built-in symbol table, and I disassembled both functions with objdump. The full annotated listings are in README.md.

# What's in build/

A Go CLI called procsearch, using only the standard library:

```text
procsearch ps [-match exact|fold|contains] [-first] NAME finds processes; -first prints just the PID.
procsearch mods -pid N [-match …] [-first] NAME finds modules in a process; -first prints just the base address.
```

Linux reads /proc. Windows uses the same snapshot APIs as the original, starting with Process32First so the first entry is checked.
Each failure gets its own error message. Exit codes are 0 found, 1 not found, 2 error (for example permission denied).
When several items match, an exact-case match wins; otherwise the first one found does.
make runs format, vet (Linux, Windows, macOS), tests and build; make windows builds bin/procsearch.exe.
I left out the parts of the original that read the game's memory and draw the overlay. I also deleted the JS/CRM files I had started in build/, and sketch-crm itself was never changed.

# Be careful

Be careful with that download folder. It's an anti-cheat-bannable cheat, run.py restarts it silently in the background, and requirements.txt lists packages (qrcode, pillow, cImage) that neither script uses. I wouldn't run anything from there
