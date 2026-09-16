// Talks to the order API.
export interface Order {
  id: string;
}

// Sends an order and returns its identifier.
export function place(order: Order): string {
  return normalise(order.id);
}

// Not exported: internal helper.
function normalise(id: string): string {
  return id.trim();
}
