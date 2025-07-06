# kisanlink-db

A Go package following the Hexagonal Architecture (Ports and Adapters) pattern, providing:

- **Database Plugins:** Support for multiple database types via pluggable adapters.
- **Connection Adapters:** Standardized connection code for each supported database.
- **Entities:** Go structs representing all Kisanlink database models.
- **Repositories:** Interfaces and implementations for CRUD and advanced operations on all entities.

## Features

- Modular and extensible database support (e.g., PostgreSQL, MySQL, SQLite, graphDBs, ...)
- Clean separation of domain logic and infrastructure
- Easy to add new database backends via plugin mechanism
- Repository interfaces for all Kisanlink models
- Standardized connection and transaction management

## Hexagonal Architecture Overview

- **Domain Layer:** Entities and repository interfaces (ports)
- **Adapters Layer:** Database-specific implementations (adapters)
- **Application Layer:** Use cases and business logic (not included in this package)

## Getting Started

### Installation

```sh
go get github.com/Kisanlink/kisanlink-db