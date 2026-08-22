#!/bin/bash
terraform import ldap_value.developers_john_doe '{"dn": "cn=developers,ou=groups,dc=example,dc=com", "attribute": "member", "value": "cn=john.doe,ou=users,dc=example,dc=com"}'
