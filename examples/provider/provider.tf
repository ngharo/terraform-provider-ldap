terraform {
  required_providers {
    ldap = {
      source = "registry.terraform.io/ngharo/ldap"
    }
  }
}

# Configure the LDAP Provider with LDAPS
provider "ldap" {
  url           = "ldaps://ldap.example.com:636"
  bind_dn       = "cn=admin,dc=example,dc=com"
  bind_password = var.ldap_password
}

# Configure with LDAP (non-TLS)
provider "ldap" {
  url           = "ldap://ldap.example.com:389"
  bind_dn       = "cn=admin,dc=example,dc=com"
  bind_password = var.ldap_password
}

# Configure with LDAPS and skip certificate verification (for self-signed certs)
provider "ldap" {
  url           = "ldaps://ldap.example.com:636"
  bind_dn       = "cn=admin,dc=example,dc=com"
  bind_password = var.ldap_password
  insecure      = true
}

# Configure using environment variables
# Set LDAP_URL, LDAP_BIND_DN, LDAP_BIND_PASSWORD, LDAP_INSECURE
provider "ldap" {
  # Configuration will be read from environment variables
}
