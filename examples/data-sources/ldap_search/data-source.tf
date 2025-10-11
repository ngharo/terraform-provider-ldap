# Basic search for all user entries
data "ldap_search" "all_users" {
  basedn = "ou=users,dc=example,dc=com"
  filter = "(objectClass=person)"
}

# Search with specific attributes
data "ldap_search" "user_emails" {
  basedn               = "ou=users,dc=example,dc=com"
  filter               = "(objectClass=person)"
  requested_attributes = ["cn", "mail", "uid"]
}

# Search with different scopes
data "ldap_search" "base_entry" {
  basedn = "cn=admin,dc=example,dc=com"
  scope  = "base"
  filter = "(objectClass=*)"
}

data "ldap_search" "direct_children" {
  basedn = "dc=example,dc=com"
  scope  = "one"
  filter = "(objectClass=organizationalUnit)"
}

# Output examples using the new structure
output "user_count" {
  description = "Total number of users found"
  value       = length(data.ldap_search.all_users.results)
}

output "user_dns" {
  description = "List of all user DNs"
  value       = [for result in data.ldap_search.all_users.results : result.dn]
}
