#!/bin/sh
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

exec slapd -f /etc/openldap/slapd.conf -h "ldap://0.0.0.0:1389" -u ldap -g ldap -d 256
