# Create a basic person entry
resource "ldap_entry" "user" {
  dn           = "cn=john.doe,ou=users,dc=example,dc=com"
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]

  attributes = {
    cn          = "john.doe"
    sn          = "Doe"
    givenName   = "John"
    mail        = "john.doe@example.com"
    description = "Software Engineer"
  }
}

# Create a group entry
resource "ldap_entry" "group" {
  dn           = "cn=developers,ou=groups,dc=example,dc=com"
  object_class = ["top", "groupOfNames"]

  attributes = {
    cn          = "developers"
    description = "Development team"
    member      = ldap_entry.user.dn
  }
}

# Create an organizational unit
resource "ldap_entry" "department" {
  dn           = "ou=engineering,dc=example,dc=com"
  object_class = ["top", "organizationalUnit"]

  attributes = {
    ou          = "engineering"
    description = "Engineering Department"
  }
}