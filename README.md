# Terraform Provider for LDAP

This Terraform provider allows you to manage LDAP (Lightweight Directory Access Protocol) entries using Terraform, enabling infrastructure as code practices for directory services.

## Features

- **LDAP Entry Management**: Create, read, update, and delete LDAP entries
- **Object Class Support**: Support for various LDAP object classes (person, organizationalPerson, inetOrgPerson, groupOfNames, etc.)
- **Attribute Management**: Flexible attribute management with validation
- **Import Support**: Import existing LDAP entries into Terraform state
- **TLS Support**: Secure connections with TLS/LDAPS
- **Environment Variables**: Configuration via environment variables

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.23 (for development)
- Access to an LDAP server (OpenLDAP, Active Directory, etc.)

## Quick Start

### 1. Configure the Provider

```terraform
terraform {
  required_providers {
    ldap = {
      source = "registry.terraform.io/ngharo/ldap"
    }
  }
}

provider "ldap" {
  host          = "ldap.example.com"
  port          = 389
  bind_dn       = "cn=admin,dc=example,dc=com"
  bind_password = var.ldap_password
}
```

### 2. Create LDAP Entries

```terraform
# Create a user entry
resource "ldap_entry" "user" {
  dn = "cn=john.doe,ou=users,dc=example,dc=com"
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]

  attributes = {
    cn        = "john.doe"
    sn        = "Doe"
    givenName = "John"
    mail      = "john.doe@example.com"
  }
}

# Create a group entry
resource "ldap_entry" "group" {
  dn = "cn=developers,ou=groups,dc=example,dc=com"
  object_class = ["top", "groupOfNames"]

  attributes = {
    cn          = "developers"
    description = "Development team"
    member      = ldap_entry.user.dn
  }
}
```

## Documentation

- [Provider Documentation](./docs/index.md)
- [ldap_entry Resource](./docs/resources/entry.md)

## Development

### Building the Provider

```bash
go build
```

### Running Tests

```bash
# Unit tests
go test ./internal/provider/

# Acceptance tests (requires running LDAP server)
TF_ACC=1 go test ./internal/provider/ -v -timeout 10m
```

### Testing with Local LDAP Server

A containerized OpenLDAP server is provided for testing:

```bash
cd test
podman build -t openldap-test -f Containerfile .
podman run -d -p 3389:1389 --name ldap-test openldap-test
```

The test server provides:
- Host: `localhost:3389`
- Bind DN: `cn=Manager,dc=example,dc=com`
- Password: `secret`
- Base DN: `dc=example,dc=com`

### Generating Documentation

```bash
cd tools
go generate
```

## Provider Configuration

| Attribute | Type | Description | Environment Variable |
|-----------|------|-------------|---------------------|
| `host` | string | LDAP server hostname/IP | `LDAP_HOST` |
| `port` | number | LDAP server port (389/636) | `LDAP_PORT` |
| `bind_dn` | string | Bind DN for authentication | `LDAP_BIND_DN` |
| `bind_password` | string | Bind password | `LDAP_BIND_PASSWORD` |
| `use_tls` | boolean | Use TLS connection | `LDAP_USE_TLS` |

## LDAP Entry Resource

The `ldap_entry` resource manages individual LDAP entries with the following attributes:

- **`dn`** (required): Distinguished Name - unique identifier for the entry
- **`object_class`** (required): List of object classes defining the entry schema
- **`attributes`** (optional): Map of LDAP attributes (excluding objectClass)

### Import

Import existing LDAP entries using their DN:

```bash
terraform import ldap_entry.user "cn=john.doe,ou=users,dc=example,dc=com"
```

## Common Object Classes

| Object Class | Required Attributes | Common Use |
|--------------|-------------------|------------|
| `person` | `cn`, `sn` | Basic person entries |
| `organizationalPerson` | `cn`, `sn` | People in organizations |
| `inetOrgPerson` | `cn`, `sn` | Internet-enabled people |
| `groupOfNames` | `cn`, `member` | Groups with members |
| `organizationalUnit` | `ou` | Organizational containers |

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run the test suite
6. Submit a pull request

## License

This project is licensed under the Mozilla Public License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- [Issues](https://github.com/ngharo/terraform-provider-ldap/issues)
- [Terraform Provider Development Documentation](https://developer.hashicorp.com/terraform/plugin)
- [LDAP Documentation](https://ldap.com/)