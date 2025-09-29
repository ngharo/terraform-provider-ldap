terraform {
  required_providers {
    ldap = {
      source = "registry.terraform.io/ngharo/ldap"
    }
  }
}

# Configure the LDAP Provider
provider "ldap" {
  host          = "ldap.example.com"
  port          = 389
  bind_dn       = "cn=admin,dc=example,dc=com"
  bind_password = var.ldap_password
  use_tls       = false
}

# Alternative configuration with TLS
provider "ldap" {
  alias         = "secure"
  host          = "ldaps.example.com"
  port          = 636
  bind_dn       = "cn=admin,dc=example,dc=com"
  bind_password = var.ldap_password
  use_tls       = true
}