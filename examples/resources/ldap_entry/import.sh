#!/bin/bash
# Import an existing LDAP entry using its DN
terraform import ldap_entry.user "cn=john.doe,ou=users,dc=example,dc=com"

# Import a group entry
terraform import ldap_entry.group "cn=developers,ou=groups,dc=example,dc=com"

# Import an organizational unit
terraform import ldap_entry.department "ou=engineering,dc=example,dc=com"
