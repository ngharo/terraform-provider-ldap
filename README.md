# Terraform Provider for LDAP

This Terraform provider allows you to manage LDAP (Lightweight Directory Access Protocol) entries using Terraform, enabling infrastructure as code practices for directory services.

This provider focuses on providing primitive LDAP operations without enforcing specific directory schemas, since LDAP schemas vary significantly
between implementations (OpenLDAP, Active Directory, etc.). Recommend wrapping inside reusable modules for common entry types (users, groups, OUs).

## Resources and Data Sources

- **`ldap_entry`**: Manage LDAP entries (Create, Read, Update, Delete)
- **`ldap_search`**: Query LDAP directories for existing entries

## Documentation

Read latest stable release documentation at https://registry.terraform.io/providers/ngharo/ldap/latest/docs

- [Provider Documentation](./docs/index.md)
- [ldap_entry Resource](./docs/resources/entry.md)
- [ldap_search Data Source](./docs/data-sources/search.md)


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
