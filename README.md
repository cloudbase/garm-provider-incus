# Incus external provider for GARM

The Incus external provider allows [garm](https://github.com/cloudbase/garm) to create runners using Incus containers and virtual machines.

## Build

Clone the repo:

```bash
git clone https://github.com/cloudbase/garm-provider-incus
```

Build the binary:

```bash
cd garm-provider-incus
go build .
```

Copy the binary on the same system where ```garm``` is running, and [point to it in the config](https://github.com/cloudbase/garm/blob/main/doc/providers.md#the-external-provider).

## Configure

The config file for this external provider is a simple toml used to configure the credentials needed to connect to your OpenStack cloud and some additional information about your environment.

A sample config file can be found [in the testdata folder](./testdata/garm-provider-incus.toml).

## Tweaking the provider

Garm supports sending opaque json encoded configs to the IaaS providers it hooks into. This allows the providers to implement some very provider specific functionality that doesn't necessarily translate well to other providers. Features that may exists on Azure, may not exist on AWS or OpenStack and vice versa.

To this end, this provider supports the following extra specs schema:

```json
{
    "$schema": "http://cloudbase.it/garm-provider-incus/schemas/extra_specs#",
    "type": "object",
    "description": "Schema defining supported extra specs for the Garm Incus Provider",
    "properties": {
        "extra_packages": {
            "type": "array",
            "description": "A list of packages that cloud-init should install on the instance.",
            "items": {
                "type": "string"
            }
        },
        "disable_updates": {
            "type": "boolean",
            "description": "Whether to disable updates when cloud-init comes online."
        },
        "enable_boot_debug": {
            "type": "boolean",
            "description": "Allows providers to set the -x flag in the runner install script."
        }
    }
}
```

An example extra specs json would look like this:

```json
{
    "disable_updates": true,
    "extra_packages": ["openssh-server", "jq"],
    "enable_boot_debug": false
}
```

To set it on an existing pool, simply run:

```bash
garm-cli pool update --extra-specs='{"disable_updates": true}' <POOL_ID>
```

You can also set a spec when creating a new pool, using the same flag.

Workers in that pool will be created taking into account the specs you set on the pool.

Aside from the above schema, this provider also supports the generic schema implemented by [`garm-provider-common`](https://github.com/cloudbase/garm-provider-common/tree/main#userdata)