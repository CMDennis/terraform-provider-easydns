---
page_title: "Five-minute sandbox quick start"
subcategory: "Guides"
description: |-
  Configure the EasyDNS sandbox and manage one disposable DNS record.
---

# Five-minute sandbox quick start

This example uses the EasyDNS sandbox. Use a sandbox-only account and a domain
that does not serve real traffic.

## 1. Export credentials

Keep credentials out of Terraform configuration and version control:

```shell
export EASYDNS_API_TOKEN='sandbox-token'
export EASYDNS_API_KEY='sandbox-key'
export EASYDNS_ENVIRONMENT='sandbox'
```

## 2. Create the configuration

Replace `your-disposable-domain.example` with a domain already present in the
sandbox.

```terraform
terraform {
  required_version = ">= 1.14.0"

  required_providers {
    easydns = {
      source = "CMDennis/easydns"
    }
  }
}

provider "easydns" {
  environment       = "sandbox"
  record_write_mode = "synchronous"
}

resource "easydns_record" "quick_start" {
  domain = "your-disposable-domain.example"
  host   = "terraform-quick-start"
  type   = "A"
  rdata  = "192.0.2.10"
  ttl    = 300
}
```

Addresses in `192.0.2.0/24` are reserved for documentation and should not be
used as real service addresses.

## 3. Plan and apply

```shell
terraform init
terraform plan -out=tfplan
terraform apply tfplan
terraform plan
```

The final plan should report no changes. The provider waits until the record is
observable before returning, even when asynchronous record writes are selected.

## 4. Clean up

```shell
terraform destroy
```

Check the EasyDNS sandbox afterward if an apply or destroy ended with an
ambiguous-write error. Do not immediately repeat an uncertain write: the first
request may have succeeded.

## Next steps

- Choose a [record write mode](record-write-modes.md).
- Copy a [record pattern](record-patterns.md).
- Read the [registrar safety guide](registrar-safety.md) before managing domains,
  delegation, registration settings, or glue.
- Use the [troubleshooting guide](troubleshooting.md) for rate limits and
  eventual consistency.
