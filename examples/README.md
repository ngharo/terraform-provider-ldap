# Examples

This directory contains examples for the Terraform LDAP Provider that demonstrate how to configure and use the provider to manage LDAP entries.

## Directory Structure

* **provider/provider.tf** - Example provider configuration showing authentication options
* **resources/ldap_entry/resource.tf** - Example LDAP entry resource configurations
* **resources/ldap_entry/import.sh** - Example import commands for existing LDAP entries

## Running Examples

To run these examples:

1. Set up an LDAP server (see test/Containerfile for a local test server)
2. Update provider configuration with your LDAP server details
3. Initialize and apply:

```bash
terraform init
terraform plan
terraform apply
```

## Test LDAP Server

A containerized OpenLDAP server is provided for testing examples:

```bash
cd ../test
podman build -t openldap-test -f Containerfile .
podman run -d -p 3389:1389 --name ldap-test openldap-test
```

Then use these provider settings:

```terraform
provider "ldap" {
  host          = "localhost"
  port          = 3389
  bind_dn       = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}
```

## LDAP Entry Examples

The examples demonstrate:

- Creating user entries with person/organizationalPerson/inetOrgPerson object classes
- Creating group entries with groupOfNames object class
- Creating organizational unit entries
- Using attribute references between resources
- Importing existing LDAP entries

Each example includes the necessary object classes and required attributes for valid LDAP entries.