# diagrammer

Reads a source tree and writes diagrams of it.

The program does two separate things. An **analyzer** turns source code into
one code graph: packages, files, types and functions, with the imports and
calls it can prove. An **emitter** turns that graph into a diagram document.
Keeping them apart is the point, because one analysis can then produce an
architecture view, a sequence view and a state view rather than only the first.

Status: **early.** The repository holds the build rules and nothing else yet.

## Build

macOS is the supported target today.

```
make build      # bin/diagrammer
make check      # fmt, vet, test
make help       # every target
```

Linux builds will come later, on a native runner rather than by cross
compiling, so that the binary that ships is the binary that was tested.

## Relationship to Archify

The ideas here were first built in
[Archify](https://github.com/tt-a1i/archify), an MIT-licensed Node.js project.
diagrammer is a separate Go program, not a fork, and is not endorsed by or
affiliated with Archify or its authors.

What that permits and what it requires is written down in
[docs/licensing.md](docs/licensing.md), and anything actually borrowed is
recorded in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## License

MIT. See [LICENSE](LICENSE).
