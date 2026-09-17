# Installing a packaged build

This document ships inside the archive, so the copy beside the binary is the
copy that describes it.

## What is in here

Two binaries, from one source tree. Which one you run decides what it can read,
and every run says which it is, so nobody learns the scope by pointing it at a
repository and wondering why the graph came back nearly empty.

- `diagrammer` reads Go and nothing else.
- `diagrammer-polyglot` reads Go, Python, Solidity and JS/TS.

There is no reason to prefer the smaller one unless you want the smaller one.
They are the same program, and the four-language build costs about 2.6 MB more.

Also `LICENSE` and `THIRD_PARTY_NOTICES.md`, which lists every module compiled
in, with its version and its licence. The second is longer for the
four-language build, because the tree-sitter runtime and four grammars are in
it.

## Which archive

`darwin_arm64` for Apple silicon, `darwin_amd64` for Intel. `uname -m` says
which you have: `arm64` or `x86_64`.

An arm64 mac will not run the Intel archive's binaries usefully, and an Intel
mac cannot run the arm64 ones at all.

## Check what you got

```
shasum -a 256 -c SHA256SUMS
```

Run it in the directory holding both the archive and `SHA256SUMS`. It prints
`OK` per archive. This tells you the file arrived intact; it is not a signature
and it does not tell you who made it.

## Install it

```
tar xzf diagrammer_<version>_darwin_<arch>.tar.gz
sudo mv diagrammer_<version>_darwin_<arch>/diagrammer* /usr/local/bin/
diagrammer version
```

Anywhere on `PATH` works. `/usr/local/bin` is the usual answer and is why
`sudo` is there.

## The minimum macOS is 12.0

Both binaries declare they need macOS 12.0 or later.

Read that as a declaration rather than a measurement. It is the floor the Go
toolchain puts on a build of its own, and the packaging sets the same one on the
C-linked build so that it does not silently inherit the macOS of whoever did the
building. Nobody has run these on a mac older than the one that packaged them.
If 12.0 turns out to be optimistic, that is a defect and worth reporting.

`sw_vers -productVersion` says which you are on.

## If macOS kills it

A binary that arrived with a quarantine flag is killed on sight, with no
message beyond the shell reporting it. That happens when a browser downloaded
the archive and Archive Utility unpacked it, because the flag is copied onto
everything inside.

```
xattr -d com.apple.quarantine /usr/local/bin/diagrammer
xattr -d com.apple.quarantine /usr/local/bin/diagrammer-polyglot
```

Unpacking with `tar` in a terminal does not do this, even when the archive
itself was quarantined. That was measured rather than assumed, and so was the
remedy above: the packaging gate quarantines a binary, watches it refuse to run,
removes the flag and watches it run.

These binaries are signed ad-hoc, which is not the same thing as notarised.
An ad-hoc signature is an identity a file gives itself, so

```
codesign --verify /usr/local/bin/diagrammer
```

answers "is this file still exactly what it was signed as" and nothing about who
signed it. Only a Developer ID signature with notarisation would satisfy
Gatekeeper without the step above, and that needs an Apple developer account.

## Driving it from a plugin

`diagrammer serve` speaks MCP over stdio:

```json
{ "command": "/usr/local/bin/diagrammer", "args": ["serve", "-root", "/path/to/work"] }
```

Every path argument is confined to `-root`, and the default is the directory the
server started in. The README has the rest.
