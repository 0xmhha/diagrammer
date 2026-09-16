"""Keeps orders."""


class Store:
    """Holds orders in memory."""

    def save(self, order):
        """Record an order under a normalised key."""
        return normalize(order.id)

    def _evict(self):
        """Not part of the public surface."""


def normalize(key):
    """Lowercase and trim an identifier."""
    return key.strip().lower()
