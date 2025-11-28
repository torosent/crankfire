---
layout: default
title: Home
---

# Crankfire Documentation

An optimized, batteries-included load testing CLI for modern APIs and real-time systems.

Crankfire helps you model realistic workloads against HTTP, WebSocket, SSE, and gRPC services (selecting one protocol mode per run) with:

- Live terminal dashboard and JSON reports
- Advanced arrival and load patterns (ramp, step, spike, Poisson arrivals)
- OAuth2/OIDC authentication helpers
- CSV/JSON data feeders with template substitution
- Protocol-aware metrics and error breakdowns

Use the navigation links below to explore the docs.

## What is Crankfire?

Crankfire is a single binary written in Go that focuses on:

- **Realistic traffic**: Arrival models, patterns, and weighted endpoints model real user behavior.
- **Deep protocol support**: HTTP, WebSocket, SSE, and gRPC share a common configuration model.
- **Operational visibility**: Interactive dashboard, progress ticker, and detailed JSON summaries.
- **Automation-friendly**: Designed to slot into CI/CD pipelines and SRE workflows.

If you have ever stitched together multiple tools or scripts to run distributed load tests, Crankfire aims to give you a single, cohesive experience.

## Who is it for?

- **Backend and API engineers** who need fast feedback on performance and regressions.
- **SREs and platform teams** running repeatable load/scalability tests.
- **QA and performance engineers** who want configurable but approachable test definitions.

## Getting Started

- **Quickstart**: See [Getting Started](getting-started.md) for installation and your first test.
- **Usage recipes**: See [Usage Examples](USAGE.md) for copy‑pasteable commands.

## Deep Dives

- [Configuration & CLI Reference](configuration.md)
- [HAR Import](har-import.md)
- [Authentication](authentication.md)
- [Data Feeders](feeders.md)
- [Protocols](protocols.md) (HTTP, WebSocket, SSE, gRPC)
- [Thresholds & Assertions](thresholds.md)
- [Thresholds Quick Reference](thresholds-quick-reference.md)
- [Dashboard & Reporting](dashboard-reporting.md)
- [Developer Guide](developer-guide.md)
