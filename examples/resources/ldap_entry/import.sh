#!/bin/bash
# Simple DN (default - imports only objectClass):
terraform import ldap_entry.user "CN=user,OU=Users,DC=example,DC=com"

# JSON with specific attributes:
terraform import ldap_entry.user '{"dn": "CN=user,OU=Users,DC=example,DC=com", "attributes": ["objectClass", "cn", "sAMAccountName",
  "userPrincipalName"]}'
