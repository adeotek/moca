# Networking

The service mesh terminates TLS at the edge. Internal calls use mTLS with
certificates rotated every 24 hours by the platform controller.

Retries are handled by the HTTP client, not by the sidecar.
