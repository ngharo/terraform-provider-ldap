# Create a basic person entry
resource "ldap_entry" "user" {
  dn           = "cn=john.doe,ou=users,dc=example,dc=com"
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]

  attributes = {
    cn          = ["john.doe"]
    sn          = ["Doe"]
    givenName   = ["John"]
    mail        = ["john.doe@example.com"]
    description = ["Software Engineer"]
  }
}

# Create a group entry with single member
resource "ldap_entry" "group" {
  dn           = "cn=developers,ou=groups,dc=example,dc=com"
  object_class = ["top", "groupOfNames"]

  attributes = {
    cn          = ["developers"]
    description = ["Development team"]
    member      = [ldap_entry.user.dn]
  }
}

# Create an organizational unit
resource "ldap_entry" "department" {
  dn           = "ou=engineering,dc=example,dc=com"
  object_class = ["top", "organizationalUnit"]

  attributes = {
    ou          = ["engineering"]
    description = ["Engineering Department"]
  }
}

# Example: User with multiple email addresses (multi-valued attribute)
resource "ldap_entry" "user_multi_email" {
  dn           = "cn=jane.smith,ou=users,dc=example,dc=com"
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]

  attributes = {
    cn        = ["jane.smith"]
    sn        = ["Smith"]
    givenName = ["Jane"]
    mail = [
      "jane.smith@example.com",
      "jane.smith@company.example.com",
      "j.smith@example.com"
    ]
    telephoneNumber = [
      "+1-555-123-4567",
      "+1-555-987-6543"
    ]
    description = ["Senior Software Engineer", "Team Lead"]
  }
}

# Example: Group with multiple members (multi-valued attribute)
resource "ldap_entry" "group_multi_member" {
  dn           = "cn=admins,ou=groups,dc=example,dc=com"
  object_class = ["top", "groupOfNames"]

  attributes = {
    cn          = ["admins"]
    description = ["System Administrators"]
    member = [
      ldap_entry.user.dn,
      ldap_entry.user_multi_email.dn,
      "cn=admin,ou=users,dc=example,dc=com"
    ]
  }
}