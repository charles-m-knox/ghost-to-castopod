# ghost-to-castopod

Takes Ghost blog subscriptions and pushes them to a Castopod server.

This leverages a database connection - Ghost recommends mysql as its supported database type, but it can technically work with any compliant driver that implements Go's `database/sql` interfaces.

This is compatible with **Ghost v5.87.1**. Compatibility with any other version is not guaranteed.

## Requirements

TBD.

## Example Usage

See [`examples/simple/README.md`](./examples/simple/README.md).

## Warnings

**Please verify the output of this application before using it**.

## Structure

In order to keep dependencies to zero (see [`go.mod`](./go.mod) - it only uses the standard library), this Go module is structured as a library that can be imported by any application.

This has the benefit of not requiring any specific SQL driver - it accepts SQL rows themselves from the `database/sql` package. You can use any compliant driver, such as sqlite or postgres - although I haven't tried anything aside from mysql, so tread carefully.

## Development notes

The `ghosttocastopod` package is the primary library for this module. Its unit test coverage is currently at **`82.5%`** and at this time, I do not intend to get it higher. The core business logic is well-tested; everything else remaining is not worth unit testing.

## Compatibility

Here is a list of all tested Ghost versions that are known to work:

- v5.87.1

If any database migrations that modify the tables used by this library occur upstream, this application will likely break.
