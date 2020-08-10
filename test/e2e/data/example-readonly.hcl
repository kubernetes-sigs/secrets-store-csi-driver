path "secret/data/foo" {
  capabilities = ["read", "list"]
}

path "secret/data/foo1" {
  capabilities = ["read", "list"]
}

path "sys/renew/*" {
  capabilities = ["update"]
}
