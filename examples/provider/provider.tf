terraform {
  required_providers {
    ldap = {
      source = "registry.terraform.io/ngharo/ldap"
    }
  }
}

# Configure the LDAP Provider
provider "ldap" {
  host          = "ldaps.example.com"
  port          = 636
  bind_dn       = "cn=admin,dc=example,dc=com"
  bind_password = var.ldap_password
  use_tls       = true
}
