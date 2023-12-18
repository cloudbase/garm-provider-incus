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

NOTE: This provider only has the `images:` remote configured. If you're coming from Incus, and you need `ubuntu` images, please use the `cloud` variant of the ubunt images (ie: `images:ubuntu/22.04/cloud`).

### Incus remotes

By default, this provider does not load any image remotes. You get to choose which remotes you add (if any). An image remote is a repository of images that Incus uses to create new instances, either virtual machines or containers. In the absence of any remote, the provider will attempt to find the image you configure for a pool of runners, on the Incus server we're connecting to. If one is present, it will be used, otherwise it will fail and you will need to configure a remote.

The sample config file in this repository has the usual default ```Incus``` remotes:

* <https://images.linuxcontainers.org> (images) - Community maintained images for various operating systems

When creating a new pool, you'll be able to specify which image you want to use. The images are referenced by ```remote_name:image_tag```. For example, if you want to launch a runner on an Ubuntu 22.04, the image name would be ```images:ubuntu/22.04/cloud```. For CentOS it would be ```images:centos/8-Stream/cloud```. You must always use the `/cloud` variant of the images, as those are the ones that have cloud-init installed and configured.

You can also create your own image remote, where you can host your own custom images. If you want to build your own images, have a look at [distrobuilder](https://github.com/lxc/distrobuilder).

Image remotes in the provider config, is a map of strings to remote settings. The name of the remote is the last bit of string in the section header. For example, the following section ```[image_remotes.images]```, defines the image remote named **images**. Use this name to reference images inside that remote.

You can also use locally uploaded images. Check out the [performance considerations](https://github.com/cloudbase/garm/blob/main/doc/performance_considerations.md) page for details on how to customize local images and use them with GARM.

### Incus Security considerations

This provider does not apply any ACLs of any kind to the instances it creates. That task remains in the responsibility of the user. [Here is a guide for creating ACLs in Incus](https://linuxcontainers.org/incus/docs/master/howto/network_acls/). You can of course use ```iptables``` or ```nftables``` to create any rules you wish. I recommend you create a separate isolated incus bridge for runners, and secure it using ACLs/iptables/nftables.

You must make sure that the code that runs as part of the workflows is trusted, and if that cannot be done, you must make sure that any malicious code that will be pulled in by the actions and run as part of a workload, is as contained as possible. There is a nice article about [securing your workflow runs here](https://blog.gitguardian.com/github-actions-security-cheat-sheet/).

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