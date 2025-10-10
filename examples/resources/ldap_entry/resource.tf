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

# Example: User with password using write-only attributes
# Write-only attributes are never stored in Terraform state (Terraform 1.11+)
resource "ldap_entry" "user_with_password" {
  dn           = "cn=secure.user,ou=users,dc=example,dc=com"
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]

  attributes = {
    cn        = ["secure.user"]
    sn        = ["User"]
    givenName = ["Secure"]
    mail      = ["secure.user@example.com"]
  }

  # Write-only attributes for sensitive data (requires Terraform 1.11+)
  attributes_wo = {
    userPassword = ["MySecretPassword123!"]
  }

  # Version number - increment to trigger password updates
  attributes_wo_version = 1
}

# Example: Active Directory user with unicodePwd
# The provider automatically encodes unicodePwd as UTF-16LE with quotes
resource "ldap_entry" "ad_user" {
  dn           = "CN=AD User,OU=Users,DC=example,DC=com"
  object_class = ["top", "person", "organizationalPerson", "user"]

  attributes = {
    cn                = ["AD User"]
    sAMAccountName    = ["aduser"]
    userPrincipalName = ["aduser@example.com"]
  }

  # unicodePwd is automatically encoded as UTF-16LE for Active Directory
  attributes_wo = {
    unicodePwd = ["Password123!"]
  }

  # Increment this version to rotate the password
  attributes_wo_version = 1
}

# Example: Rotating a password by incrementing the version
resource "ldap_entry" "user_password_rotation" {
  dn           = "cn=rotating.user,ou=users,dc=example,dc=com"
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]

  attributes = {
    cn        = ["rotating.user"]
    sn        = ["User"]
    givenName = ["Rotating"]
  }

  attributes_wo = {
    userPassword = ["NewPassword456!"]
  }

  # Increment from 1 -> 2 -> 3, etc. to trigger password updates
  # Only when this value changes will attributes_wo be sent to LDAP
  attributes_wo_version = 2
}