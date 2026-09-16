# Third-party notices

diagrammer's own code is released under the MIT license in `LICENSE`. This file
records everything in this repository that came from somewhere else, and the
terms that came with it.

Every entry must name what was taken, where it came from, and under what
licence. An entry is added in the same commit that brings the material in, not
afterwards.

## Archify

diagrammer reimplements ideas first built in
[Archify](https://github.com/tt-a1i/archify), which is distributed under the
MIT license. Archify is a Node.js project; diagrammer is a separate Go program
and is not a fork.

Reimplementing behaviour observed in an MIT project creates no obligation on
its own. Copying material does. The table below records what was copied, if
anything.

| Material | Taken from | Status |
|---|---|---|
| (none yet) | | |

When a row is added here, the Archify copyright notice below must ship with the
distribution, because MIT requires it for copies and substantial portions.

```
MIT License

Copyright (c) 2026 tt-a1i (Archify)
Copyright (c) 2025 Cocoon AI

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Go modules compiled into the binary

The table above records material copied into this repository. The modules below
are not in the repository, but they are compiled into the binary that ships, and
every one of their licences requires the notice to travel with a distribution.
They are listed here for that reason.

Versions are the ones `go.mod` pins. All of these licences are permissive and
none of them restricts what this project may do; the obligation is attribution.

| Module | Version | Licence |
|---|---|---|
| github.com/santhosh-tekuri/jsonschema/v6 | v6.0.3 | Apache-2.0 |
| github.com/modelcontextprotocol/go-sdk | v1.8.0 | Apache-2.0 and MIT (see below) |
| github.com/google/jsonschema-go | v0.4.3 | MIT |
| github.com/segmentio/encoding | v0.5.4 | MIT |
| github.com/segmentio/asm | v1.1.3 | MIT |
| github.com/yosida95/uritemplate/v3 | v3.0.2 | BSD-3-Clause |
| golang.org/x/oauth2 | v0.35.0 | BSD-3-Clause |
| golang.org/x/sync | v0.20.0 | BSD-3-Clause |
| golang.org/x/sys | v0.41.0 | BSD-3-Clause |
| golang.org/x/text | v0.14.0 | BSD-3-Clause |
| golang.org/x/time | v0.15.0 | BSD-3-Clause |

The MCP Go SDK is mid-transition from MIT to Apache-2.0. New contributions are
Apache-2.0; contributions whose authors have not consented to relicensing remain
MIT. Its `LICENSE` file states this, and both licences are satisfied by the row
above plus the copy shipped with the module.

Four of these arrive only because the MCP SDK supports transports this project
does not use: oauth2, uritemplate, sync and time are reached through its HTTP
and authorisation paths. They are compiled in and never executed on the stdio
transport the server runs on. They are listed anyway, because what ships is what
has to be attributed, not what runs.

This table is regenerated rather than remembered. `go list -deps ./cmd/diagrammer`
names every package the binary links, and anything new in that list belongs here
in the same commit that adds it.

## No affiliation

diagrammer is not endorsed by or affiliated with Archify or its authors. Names
and trademarks remain the property of their owners.
