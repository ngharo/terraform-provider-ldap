# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This project is a Terraform provider that implements basic LDAP CRUD operations. Generally, this
provider is designed to provide a primitive ldap_entry resource that users can wrap in a module
to provide more user friendly functionality to meet their needs.

## Development Setup

Since this is a new Terraform provider project, the typical setup would involve:

- Go development environment (minimum Go 1.23.0 based on previous notes)
- Terraform development tools
- LDAP testing environment

## Expected Project Structure

When the codebase is developed, it will likely follow standard Terraform provider patterns:
- Main provider code in root or `internal/provider/`
- Resource and data source implementations
- Acceptance tests in `*_test.go` files
- Documentation in `docs/` or `website/`
- Example configurations in `examples/`
- Test environment using podman in `test/`

## Build Commands

Once the project is initialized:
- `make` - Format and build project
- `make deps` - Get all dependencies
- `make test` - Run all tests

## Test Commands

- `go test -v ./...` - Run all tests verbosely
- `go test -v -run=TestName` - Run a specific test by name
- Acceptance tests will likely use `TF_ACC=1` environment variable

## Code Style

- Use `goimports` for formatting (run via `make`)
- Follow standard Go formatting conventions
- Group imports: standard library first, then third-party
- Use PascalCase for exported types/methods, camelCase for variables
- Add comments for public API and complex logic
- Place related functionality in logically named files

## Error Handling

- Use custom `Error` type with detailed context
- Include error wrapping with `Unwrap()` method
- Return errors with proper context information (line, position)

## Testing

- Write table-driven tests with clear input/output expectations
- Use package `tpl_test` for external testing perspective
- Include detailed error messages (expected vs. actual)
- Test every exported function and error case
- Terraform providers require comprehensive acceptance tests

## Dependencies

- Minimum Go version: 1.23.0
- External dependencies managed through go modules
- Will likely depend on Terraform Plugin SDK and LDAP libraries

## Modernization Notes

- Use `errors.Is()` and `errors.As()` for error checking
- Replace `interface{}` with `any` type alias
- Replace type assertions with type switches where appropriate
- Use generics for type-safe operations
- Implement context cancellation handling for long operations
- Add proper docstring comments for exported functions and types
- Use log/slog for structured logging
- Add linting and static analysis tools
