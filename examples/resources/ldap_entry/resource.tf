# Create a basic person entry
resource "ldap_entry" "user" {
  dn = "cn=john.doe,ou=users,dc=example,dc=com"
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn          = ["john.doe"]
    sn          = ["Doe"]
    givenName   = ["John"]
    mail        = ["john.doe@example.com"]
    description = ["Software Engineer"]
  }
}

# Create a group entry
resource "ldap_entry" "group" {
  dn = "cn=developers,ou=groups,dc=example,dc=com"
  attributes = {
    objectClass = ["top", "groupOfNames"]
    cn          = ["developers"]
    description = ["Development team"]
    member = [
      ldap_entry.user.dn,
      "cn=admin,ou=users,dc=example,dc=com",
    ]
  }
}

# Create an organizational unit
resource "ldap_entry" "department" {
  dn = "ou=engineering,dc=example,dc=com"
  attributes = {
    objectClass = ["top", "organizationalUnit"]
    ou          = ["engineering"]
    description = ["Engineering Department"]
  }
}

locals {
  // userAccountControl tags
  // Enforcment based on the sum of tags
  // https://learn.microsoft.com/en-us/troubleshoot/windows-server/active-directory/useraccountcontrol-manipulate-account-properties
  tags = {
    ACCOUNTDISABLE                 = 2
    HOMEDIR_REQUIRED               = 8
    LOCKOUT                        = 16
    PASSWD_NOTREQD                 = 32
    PASSWD_CANT_CHANGE             = 64
    ENCRYPTED_TEXT_PWD_ALLOWED     = 128
    TEMP_DUPLICATE_ACCOUNT         = 256
    NORMAL_ACCOUNT                 = 512
    INTERDOMAIN_TRUST_ACCOUNT      = 2048
    WORKSTATION_TRUST_ACCOUNT      = 4096
    SERVER_TRUST_ACCOUNT           = 8192
    DONT_EXPIRE_PASSWORD           = 65536
    MNS_LOGON_ACCOUNT              = 131072
    SMARTCARD_REQUIRED             = 262144
    TRUSTED_FOR_DELEGATION         = 524288
    NOT_DELEGATED                  = 1048576
    USE_DES_KEY_ONLY               = 2097152
    DONT_REQ_PREAUTH               = 4194304
    PASSWORD_EXPIRED               = 8388608
    TRUSTED_TO_AUTH_FOR_DELEGATION = 16777216
  }
}
# Example: Active Directory user with unicodePwd
# The provider automatically encodes unicodePwd as UTF-16LE with quotes
resource "ldap_entry" "ad_user" {
  dn = "CN=AD User,OU=Users,DC=example,DC=com"
  attributes = {
    objectClass        = ["top", "person", "organizationalPerson", "user"]
    cn                 = ["AD User"]
    sAMAccountName     = ["aduser"]
    userPrincipalName  = ["aduser@example.com"]
    accountExpires     = ["0"]
    userAccountControl = [tostring(local.tags.NORMAL_ACCOUNT + local.tags.DONT_EXPIRE_PASSWORD)]
  }

  # unicodePwd is automatically encoded as UTF-16LE for Active Directory
  attributes_wo = {
    unicodePwd = ["Password123!"]
  }

  # Increment this version to rotate the password
  attributes_wo_version = 1
}

# Example: Group with null member attribute for external management
# This creates a group where membership is managed by external systems
# (e.g., scripts, AD tools, other automation) but Terraform can read the current members
resource "ldap_entry" "ad_group" {
  dn = "CN=AppUsers,OU=Groups,DC=example,DC=com"
  attributes = {
    objectClass = ["top", "group"]
    cn          = ["AppUsers"]
    description = ["Application users - membership managed externally"]
    member      = null # Not managed by Terraform, but will be read from LDAP
  }
}

# Reference the externally-managed member list in outputs or other resources
output "current_members" {
  value = ldap_entry.ad_group.attributes.member
}
