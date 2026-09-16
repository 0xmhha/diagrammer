// Re-exports the client so the import graph has an edge to follow.
import { place } from "./client";
// From outside the tree: counted at the boundary, never drawn as an edge.
import { join } from "node:path";

// Forward an order to the client.
export function forward(id: string): string {
  return place({ id });
}
