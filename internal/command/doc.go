// Package command holds the capabilities the binary offers, independent of how
// they are reached.
//
// The CLI and the MCP server are two faces of one set of operations, and
// neither may gain a capability the other lacks. That is a contract rather than
// an aspiration, so it is arranged to be checkable: every operation is declared
// once here, both surfaces are built from that declaration, and a test asserts
// they cover the same set with the same arguments.
//
// Keeping the operations here also keeps the argument names honest. Once 0.1.0
// ships, plugins bind to the MCP tool and argument names, so a rename is a
// breaking change rather than a tidy-up, and it should be as visible as one.
package command
