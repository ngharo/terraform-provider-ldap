# The group below is assumed to already exist (e.g. provisioned by another
# module/workspace, or bootstrapped outside of Terraform) with at least one
# seed member to satisfy the groupOfNames schema. Do not also manage the
# `member` attribute of this DN with an `ldap_entry` resource: `ldap_entry`
# treats its `attributes` map as the complete set of values, and would fight
# `ldap_value` over any values it doesn't know about.
locals {
  developers_group_dn = "cn=developers,ou=groups,dc=example,dc=com"
}

# Assert that individual users are members of the group, without any one
# resource owning the full member list
resource "ldap_value" "developers_john_doe" {
  dn        = local.developers_group_dn
  attribute = "member"
  value     = "cn=john.doe,ou=users,dc=example,dc=com"
}

resource "ldap_value" "developers_jane_doe" {
  dn        = local.developers_group_dn
  attribute = "member"
  value     = "cn=jane.doe,ou=users,dc=example,dc=com"
}
