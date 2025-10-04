# Terraform Provider for LDAP

This Terraform provider allows you to manage LDAP (Lightweight Directory Access Protocol) entries using Terraform, enabling infrastructure as code practices for directory services.

This provider focuses on providing primitive LDAP operations without enforcing specific directory schemas, since LDAP schemas vary significantly
between implementations (OpenLDAP, Active Directory, etc.). Recommend wrapping inside reusable modules for common entry types (users, groups, OUs).

## Resources and Data Sources

- **`ldap_entry`**: Manage LDAP entries (Create, Read, Update, Delete)
- **`ldap_search`**: Query LDAP directories for existing entries

## Documentation

- [Provider Documentation](./docs/index.md)
- [ldap_entry Resource](./docs/resources/entry.md)
- [ldap_search Data Source](./docs/data-sources/search.md)

### Import

Import existing LDAP entries using their DN:

```bash
terraform import ldap_entry.user "cn=john.doe,ou=users,dc=example,dc=com"
```

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

LDAP attributes are inherently multi-valued. This provider supports this by requiring all attributes to be specified as lists of strings:

```terraform
# Create a user entry
resource "ldap_entry" "user" {
  dn = "cn=john.doe,ou=users,dc=example,dc=com"
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]

  attributes = {
    cn        = ["john.doe"]
    sn        = ["Doe"]
    givenName = ["John"]
    mail = [
      "john.doe@example.com",
      "john.doe@company.example.com",
      "j.doe@example.com",
    ]
  }
}

# Create a group entry
resource "ldap_entry" "group" {
  dn = "cn=developers,ou=groups,dc=example,dc=com"
  object_class = ["top", "groupOfNames"]

  attributes = {
    cn          = ["developers"]
    description = ["Development team"]
    member      = [
      ldap_entry.user.dn,
      "cn=admin,ou=users,dc=example,dc=com",
    ]
  }
}
```

### 3. LDAP Search Data Source

Query your LDAP directory to find entries and access their attributes:

```terraform
# Search for all users with email addresses
data "ldap_search" "users_with_email" {
  basedn = "ou=users,dc=example,dc=com"
  filter = "(&(objectClass=person)(mail=*))"
  requested_attributes = ["cn", "mail", "telephoneNumber"]
}

# Output user information
# Note: attributes are always returned as lists
output "user_emails" {
  value = [
    for result in data.ldap_search.users_with_email.results : {
      name   = result.attributes.cn[0]
      emails = result.attributes.mail[0]
      phones = try(result.attributes.telephoneNumber[0], "")
    }
  ]
}
```

## Development

### Building the Provider

```bash
go build
```

### Running Tests

To run launch a containerized LDAP server and run the tests:

```bash
make test
```

A containerized OpenLDAP server is provided for testing (requires podman):

```bash
make testenv
```

The test server provides:
- Host: `localhost:3389`
- Bind DN: `cn=Manager,dc=example,dc=com`
- Password: `secret`
- Base DN: `dc=example,dc=com`

### Generating Documentation

```bash
make generate
```

## License

This project is licensed under the Mozilla Public License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- [Issues](https://github.com/ngharo/terraform-provider-ldap/issues)
- [Terraform Provider Development Documentation](https://developer.hashicorp.com/terraform/plugin)
