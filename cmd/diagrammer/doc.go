// Command diagrammer turns a source tree into UML diagrams, in four stages that
// each run on their own.
//
//	graph     stage 1   AST parse to a code graph
//	          stage 2   a plugin's skill analyses that graph with an LLM and
//	                    returns a UML codegraph; the binary has no subcommand
//	                    for it and never calls a model itself
//	validate  stage 2 boundary   accept or refuse the returned UML model
//	compose   stage 3   UML model to diagram-source documents
//	render    stage 4   document to a self-contained HTML page
//	serve               the same capabilities as a local MCP server
//
// No single command infers its stage from the shape of the file it is handed.
// Stage separation is a requirement, and a command that guesses makes the
// boundaries invisible exactly where they matter most.
package main
