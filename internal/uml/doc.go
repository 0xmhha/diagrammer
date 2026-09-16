// Package uml holds the Go form of the stage-2 UML codegraph.
//
// These types mirror schemas/codegraph.schema.json, which is the contract. They
// are checked against it by TestSchemaBinding; when the two disagree the schema
// is right and this file is what gets corrected.
//
// Nothing here is placed. A Model says what the diagram means; where anything
// goes is stage 3's decision.
package uml
